package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gpu-telemetry-pipeline/mq/internal/broker"
	"github.com/gpu-telemetry-pipeline/mq/internal/config"
	"github.com/gpu-telemetry-pipeline/mq/pkg/models"
	"github.com/rs/zerolog"
)

func newTestHandler() (*Handler, *broker.Broker) {
	cfg := config.Config{
		MaxQueueSize:    100,
		MaxMessageAge:   5 * time.Minute,
		AckTimeout:      30 * time.Second,
		CleanupInterval: 1 * time.Second,
	}
	logger := zerolog.Nop()
	b := broker.NewBroker(cfg, logger)
	h := NewHandler(b, logger)
	return h, b
}

func TestHandler_Health(t *testing.T) {
	h, b := newTestHandler()
	defer b.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", resp.Status)
	}
}

func TestHandler_Publish(t *testing.T) {
	h, b := newTestHandler()
	defer b.Close()

	t.Run("success", func(t *testing.T) {
		body := `{"payload": {"data": "test"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/topics/test-topic/messages", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "test-topic")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.Publish(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", w.Code)
		}

		var resp models.PublishResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.Status != "ok" {
			t.Errorf("Expected status 'ok', got %s", resp.Status)
		}

		if resp.MessageID == "" {
			t.Error("Expected message_id to be set")
		}
	})

	t.Run("missing topic", func(t *testing.T) {
		body := `{"payload": {"data": "test"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/topics//messages", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.Publish(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("invalid body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/topics/test-topic/messages", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "test-topic")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.Publish(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

func TestHandler_Subscribe(t *testing.T) {
	h, b := newTestHandler()
	defer b.Close()

	// First publish to create topic
	msg := &models.Message{
		ID:        "msg-1",
		Topic:     "test-topic",
		Payload:   json.RawMessage(`{}`),
		Timestamp: time.Now(),
	}
	b.Publish("test-topic", msg)

	t.Run("success", func(t *testing.T) {
		body := `{"consumer_id": "consumer-1", "group": "group-1"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/topics/test-topic/subscribe", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "test-topic")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.Subscribe(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp models.SubscribeResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.Status != "subscribed" {
			t.Errorf("Expected status 'subscribed', got %s", resp.Status)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		body := `{"consumer_id": ""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/topics/test-topic/subscribe", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "test-topic")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.Subscribe(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

func TestHandler_Consume(t *testing.T) {
	h, b := newTestHandler()
	defer b.Close()

	// Setup: publish and subscribe
	for i := 0; i < 5; i++ {
		msg := &models.Message{
			ID:        "msg-" + string(rune('0'+i)),
			Topic:     "test-topic",
			Payload:   json.RawMessage(`{}`),
			Timestamp: time.Now(),
		}
		b.Publish("test-topic", msg)
	}
	b.Subscribe("test-topic", "consumer-1", "group-1")

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topics/test-topic/messages?consumer_id=consumer-1&group=group-1&max_messages=3", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "test-topic")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.Consume(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp models.ConsumeResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if len(resp.Messages) != 3 {
			t.Errorf("Expected 3 messages, got %d", len(resp.Messages))
		}
	})

	t.Run("missing params", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/topics/test-topic/messages", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "test-topic")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.Consume(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

func TestHandler_Ack(t *testing.T) {
	h, b := newTestHandler()
	defer b.Close()

	// Setup
	msg := &models.Message{
		ID:        "msg-1",
		Topic:     "test-topic",
		Payload:   json.RawMessage(`{}`),
		Timestamp: time.Now(),
	}
	b.Publish("test-topic", msg)
	b.Subscribe("test-topic", "consumer-1", "group-1")
	b.Consume("test-topic", "consumer-1", "group-1", 1)

	t.Run("success", func(t *testing.T) {
		body := `{"consumer_id": "consumer-1", "message_ids": ["msg-1"]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/topics/test-topic/ack", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "test-topic")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.Ack(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp models.AckResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if resp.Status != "ok" {
			t.Errorf("Expected status 'ok', got %s", resp.Status)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		body := `{"consumer_id": ""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/topics/test-topic/ack", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "test-topic")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.Ack(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

func TestHandler_ListTopics(t *testing.T) {
	h, b := newTestHandler()
	defer b.Close()

	// Create topics
	for _, topic := range []string{"topic-a", "topic-b"} {
		msg := &models.Message{ID: "1", Topic: topic, Payload: json.RawMessage(`{}`), Timestamp: time.Now()}
		b.Publish(topic, msg)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/topics", nil)
	w := httptest.NewRecorder()

	h.ListTopics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.TopicListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(resp.Topics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(resp.Topics))
	}
}

func TestHandler_GetTopicStats(t *testing.T) {
	h, b := newTestHandler()
	defer b.Close()

	// Create topic with messages
	for i := 0; i < 5; i++ {
		msg := &models.Message{ID: "msg-" + string(rune('0'+i)), Topic: "test-topic", Payload: json.RawMessage(`{}`), Timestamp: time.Now()}
		b.Publish("test-topic", msg)
	}

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/topics/test-topic/stats", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "test-topic")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.GetTopicStats(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var stats models.TopicStats
		if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if stats.QueueSize != 5 {
			t.Errorf("Expected queue size 5, got %d", stats.QueueSize)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/topics/non-existent/stats", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "non-existent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.GetTopicStats(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

func TestHandler_GetMessages(t *testing.T) {
	h, b := newTestHandler()
	defer b.Close()

	for i := 0; i < 10; i++ {
		msg := &models.Message{ID: "msg-" + string(rune('0'+i)), Topic: "test-topic", Payload: json.RawMessage(`{}`), Timestamp: time.Now()}
		b.Publish("test-topic", msg)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/topics/test-topic/messages?limit=5", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("topic", "test-topic")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.GetMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.MessagesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Count != 5 {
		t.Errorf("Expected count 5, got %d", resp.Count)
	}
}

func TestHandler_DeleteMessage(t *testing.T) {
	h, b := newTestHandler()
	defer b.Close()

	msg := &models.Message{ID: "msg-to-delete", Topic: "test-topic", Payload: json.RawMessage(`{}`), Timestamp: time.Now()}
	b.Publish("test-topic", msg)

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/topics/test-topic/messages/msg-to-delete", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "test-topic")
		rctx.URLParams.Add("messageID", "msg-to-delete")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.DeleteMessage(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/topics/test-topic/messages/non-existent", nil)

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("topic", "test-topic")
		rctx.URLParams.Add("messageID", "non-existent")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		h.DeleteMessage(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

func TestHandler_PurgeMessages(t *testing.T) {
	h, b := newTestHandler()
	defer b.Close()

	for i := 0; i < 5; i++ {
		msg := &models.Message{ID: "msg-" + string(rune('0'+i)), Topic: "test-topic", Payload: json.RawMessage(`{}`), Timestamp: time.Now()}
		b.Publish("test-topic", msg)
	}

	req := httptest.NewRequest(http.MethodDelete, "/admin/topics/test-topic/messages", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("topic", "test-topic")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.PurgeMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.PurgeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.MessagesDeleted != 5 {
		t.Errorf("Expected 5 deleted, got %d", resp.MessagesDeleted)
	}
}

func TestHandler_GetConsumers(t *testing.T) {
	h, b := newTestHandler()
	defer b.Close()

	msg := &models.Message{ID: "msg-1", Topic: "test-topic", Payload: json.RawMessage(`{}`), Timestamp: time.Now()}
	b.Publish("test-topic", msg)
	b.Subscribe("test-topic", "consumer-1", "group-1")

	req := httptest.NewRequest(http.MethodGet, "/admin/topics/test-topic/consumers", nil)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("topic", "test-topic")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.GetConsumers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp models.ConsumersResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(resp.Consumers) != 1 {
		t.Errorf("Expected 1 consumer, got %d", len(resp.Consumers))
	}
}
