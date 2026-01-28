// Package api provides HTTP handlers for the MQ broker.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gpu-telemetry-pipeline/mq/internal/broker"
	"github.com/gpu-telemetry-pipeline/mq/pkg/models"
	"github.com/rs/zerolog"
)

// Handler contains HTTP handlers for the MQ API.
type Handler struct {
	broker *broker.Broker
	logger zerolog.Logger
}

// NewHandler creates a new handler instance.
func NewHandler(b *broker.Broker, logger zerolog.Logger) *Handler {
	return &Handler{
		broker: b,
		logger: logger.With().Str("component", "api").Logger(),
	}
}

// Health returns the health status.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// Publish handles message publishing.
func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		h.errorResponse(w, http.StatusBadRequest, "topic is required")
		return
	}

	var req models.PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Create message
	msg := &models.Message{
		ID:        req.ID,
		Topic:     topic,
		Payload:   req.Payload,
		Timestamp: req.Timestamp,
	}

	// Generate ID if not provided
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}

	// Use current time if not provided
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	if err := h.broker.Publish(topic, msg); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(models.PublishResponse{
		Status:    "ok",
		MessageID: msg.ID,
	})
}

// Subscribe handles consumer subscription.
func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		h.errorResponse(w, http.StatusBadRequest, "topic is required")
		return
	}

	var req models.SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ConsumerID == "" || req.Group == "" {
		h.errorResponse(w, http.StatusBadRequest, "consumer_id and group are required")
		return
	}

	if err := h.broker.Subscribe(topic, req.ConsumerID, req.Group); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.SubscribeResponse{
		Status: "subscribed",
		Topic:  topic,
		Group:  req.Group,
	})
}

// Consume handles message consumption.
func (h *Handler) Consume(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		h.errorResponse(w, http.StatusBadRequest, "topic is required")
		return
	}

	consumerID := r.URL.Query().Get("consumer_id")
	group := r.URL.Query().Get("group")
	maxMessagesStr := r.URL.Query().Get("max_messages")

	if consumerID == "" || group == "" {
		h.errorResponse(w, http.StatusBadRequest, "consumer_id and group are required")
		return
	}

	maxMessages := 10
	if maxMessagesStr != "" {
		if n, err := strconv.Atoi(maxMessagesStr); err == nil && n > 0 {
			maxMessages = n
		}
	}

	messages, err := h.broker.Consume(topic, consumerID, group, maxMessages)
	if err != nil {
		h.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.ConsumeResponse{
		Messages: messages,
	})
}

// Ack handles message acknowledgment.
func (h *Handler) Ack(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		h.errorResponse(w, http.StatusBadRequest, "topic is required")
		return
	}

	var req models.AckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ConsumerID == "" || len(req.MessageIDs) == 0 {
		h.errorResponse(w, http.StatusBadRequest, "consumer_id and message_ids are required")
		return
	}

	acked, err := h.broker.Ack(topic, req.ConsumerID, req.MessageIDs)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.AckResponse{
		Status: "ok",
		Acked:  acked,
	})
}

// --- Admin Endpoints ---

// ListTopics returns all topics.
func (h *Handler) ListTopics(w http.ResponseWriter, r *http.Request) {
	topics := h.broker.ListTopics()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"topics": topics,
	})
}

// GetTopicStats returns statistics for a topic.
func (h *Handler) GetTopicStats(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		h.errorResponse(w, http.StatusBadRequest, "topic is required")
		return
	}

	stats := h.broker.GetTopicStats(topic)
	if stats == nil {
		h.errorResponse(w, http.StatusNotFound, "topic not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// GetMessages returns messages from a topic (admin).
func (h *Handler) GetMessages(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		h.errorResponse(w, http.StatusBadRequest, "topic is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	messages := h.broker.GetMessages(topic, limit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"topic":    topic,
		"count":    len(messages),
		"messages": messages,
	})
}

// DeleteMessage deletes a specific message.
func (h *Handler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	messageID := chi.URLParam(r, "messageID")

	if topic == "" || messageID == "" {
		h.errorResponse(w, http.StatusBadRequest, "topic and message_id are required")
		return
	}

	deleted := h.broker.DeleteMessage(topic, messageID)
	if !deleted {
		h.errorResponse(w, http.StatusNotFound, "message not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "deleted",
		"message_id": messageID,
	})
}

// PurgeMessages deletes all messages from a topic.
func (h *Handler) PurgeMessages(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		h.errorResponse(w, http.StatusBadRequest, "topic is required")
		return
	}

	deleted := h.broker.PurgeMessages(topic)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "purged",
		"messages_deleted": deleted,
	})
}

// GetConsumers returns consumers for a topic.
func (h *Handler) GetConsumers(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		h.errorResponse(w, http.StatusBadRequest, "topic is required")
		return
	}

	consumers := h.broker.GetConsumers(topic)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"topic":     topic,
		"consumers": consumers,
	})
}

// errorResponse sends an error response.
func (h *Handler) errorResponse(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error: message,
		Code:  code,
	})
}
