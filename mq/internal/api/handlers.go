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

// Health godoc
// @Summary      Health check
// @Description  Returns the health status of the MQ broker
// @Tags         Health
// @Produce      json
// @Success      200  {object}  models.HealthResponse
// @Router       /health [get]
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.HealthResponse{
		Status: "healthy",
		Time:   time.Now().Format(time.RFC3339),
	})
}

// Publish godoc
// @Summary      Publish a message
// @Description  Publishes a message to a topic
// @Tags         Messages
// @Accept       json
// @Produce      json
// @Param        topic    path      string                true  "Topic name"
// @Param        message  body      models.PublishRequest true  "Message to publish"
// @Success      201      {object}  models.PublishResponse
// @Failure      400      {object}  models.ErrorResponse
// @Failure      500      {object}  models.ErrorResponse
// @Router       /api/v1/topics/{topic}/messages [post]
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

// Subscribe godoc
// @Summary      Subscribe to a topic
// @Description  Subscribes a consumer to a topic within a consumer group
// @Tags         Consumers
// @Accept       json
// @Produce      json
// @Param        topic        path      string                  true  "Topic name"
// @Param        subscription body      models.SubscribeRequest true  "Subscription details"
// @Success      200          {object}  models.SubscribeResponse
// @Failure      400          {object}  models.ErrorResponse
// @Failure      500          {object}  models.ErrorResponse
// @Router       /api/v1/topics/{topic}/subscribe [post]
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

// Consume godoc
// @Summary      Consume messages
// @Description  Consumes messages from a topic for a consumer
// @Tags         Messages
// @Produce      json
// @Param        topic        path      string  true   "Topic name"
// @Param        consumer_id  query     string  true   "Consumer ID"
// @Param        group        query     string  true   "Consumer group"
// @Param        max_messages query     int     false  "Maximum messages to consume (default 10)"
// @Success      200          {object}  models.ConsumeResponse
// @Failure      400          {object}  models.ErrorResponse
// @Router       /api/v1/topics/{topic}/messages [get]
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

// Ack godoc
// @Summary      Acknowledge messages
// @Description  Acknowledges receipt of messages by a consumer
// @Tags         Messages
// @Accept       json
// @Produce      json
// @Param        topic  path      string            true  "Topic name"
// @Param        ack    body      models.AckRequest true  "Acknowledgment details"
// @Success      200    {object}  models.AckResponse
// @Failure      400    {object}  models.ErrorResponse
// @Failure      500    {object}  models.ErrorResponse
// @Router       /api/v1/topics/{topic}/ack [post]
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

// ListTopics godoc
// @Summary      List all topics
// @Description  Returns a list of all topics in the broker
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  models.TopicListResponse
// @Router       /admin/topics [get]
func (h *Handler) ListTopics(w http.ResponseWriter, r *http.Request) {
	topics := h.broker.ListTopics()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.TopicListResponse{
		Topics: topics,
	})
}

// GetTopicStats godoc
// @Summary      Get topic statistics
// @Description  Returns statistics for a specific topic
// @Tags         Admin
// @Produce      json
// @Param        topic  path      string  true  "Topic name"
// @Success      200    {object}  models.TopicStats
// @Failure      404    {object}  models.ErrorResponse
// @Router       /admin/topics/{topic}/stats [get]
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

// GetMessages godoc
// @Summary      Get messages from topic (admin)
// @Description  Returns messages from a topic for administrative viewing
// @Tags         Admin
// @Produce      json
// @Param        topic  path      string  true   "Topic name"
// @Param        limit  query     int     false  "Maximum messages to return (default 100)"
// @Success      200    {object}  models.MessagesResponse
// @Router       /admin/topics/{topic}/messages [get]
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
	json.NewEncoder(w).Encode(models.MessagesResponse{
		Topic:    topic,
		Count:    len(messages),
		Messages: messages,
	})
}

// DeleteMessage godoc
// @Summary      Delete a message
// @Description  Deletes a specific message from a topic
// @Tags         Admin
// @Produce      json
// @Param        topic      path      string  true  "Topic name"
// @Param        messageID  path      string  true  "Message ID"
// @Success      200        {object}  models.DeleteResponse
// @Failure      404        {object}  models.ErrorResponse
// @Router       /admin/topics/{topic}/messages/{messageID} [delete]
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
	json.NewEncoder(w).Encode(models.DeleteResponse{
		Status:    "deleted",
		MessageID: messageID,
	})
}

// PurgeMessages godoc
// @Summary      Purge all messages
// @Description  Deletes all messages from a topic
// @Tags         Admin
// @Produce      json
// @Param        topic  path      string  true  "Topic name"
// @Success      200    {object}  models.PurgeResponse
// @Router       /admin/topics/{topic}/messages [delete]
func (h *Handler) PurgeMessages(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		h.errorResponse(w, http.StatusBadRequest, "topic is required")
		return
	}

	deleted := h.broker.PurgeMessages(topic)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.PurgeResponse{
		Status:          "purged",
		MessagesDeleted: deleted,
	})
}

// GetConsumers godoc
// @Summary      Get topic consumers
// @Description  Returns all consumers for a topic
// @Tags         Admin
// @Produce      json
// @Param        topic  path      string  true  "Topic name"
// @Success      200    {object}  models.ConsumersResponse
// @Router       /admin/topics/{topic}/consumers [get]
func (h *Handler) GetConsumers(w http.ResponseWriter, r *http.Request) {
	topic := chi.URLParam(r, "topic")
	if topic == "" {
		h.errorResponse(w, http.StatusBadRequest, "topic is required")
		return
	}

	consumers := h.broker.GetConsumers(topic)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.ConsumersResponse{
		Topic:     topic,
		Consumers: consumers,
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
