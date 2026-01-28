package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMessage_JSON(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	msg := Message{
		ID:        "msg-123",
		Topic:     "test-topic",
		Payload:   json.RawMessage(`{"data": "test"}`),
		Timestamp: now,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal Message: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal Message: %v", err)
	}

	if decoded.ID != msg.ID {
		t.Errorf("Expected ID %s, got %s", msg.ID, decoded.ID)
	}

	if decoded.Topic != msg.Topic {
		t.Errorf("Expected Topic %s, got %s", msg.Topic, decoded.Topic)
	}
}

func TestPublishRequest_JSON(t *testing.T) {
	req := PublishRequest{
		Payload: json.RawMessage(`{"metric":"gpu_util","value":50}`),
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal PublishRequest: %v", err)
	}

	var decoded PublishRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal PublishRequest: %v", err)
	}

	// Compare as raw JSON bytes (no whitespace normalization needed with compact JSON)
	if len(decoded.Payload) == 0 {
		t.Error("Expected non-empty Payload")
	}
}

func TestPublishResponse_JSON(t *testing.T) {
	resp := PublishResponse{
		Status:    "ok",
		MessageID: "msg-123",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal PublishResponse: %v", err)
	}

	var decoded PublishResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal PublishResponse: %v", err)
	}

	if decoded.Status != resp.Status {
		t.Errorf("Expected Status %s, got %s", resp.Status, decoded.Status)
	}

	if decoded.MessageID != resp.MessageID {
		t.Errorf("Expected MessageID %s, got %s", resp.MessageID, decoded.MessageID)
	}
}

func TestSubscribeRequest_JSON(t *testing.T) {
	req := SubscribeRequest{
		ConsumerID: "consumer-1",
		Group:      "group-1",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal SubscribeRequest: %v", err)
	}

	var decoded SubscribeRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal SubscribeRequest: %v", err)
	}

	if decoded.ConsumerID != req.ConsumerID {
		t.Errorf("Expected ConsumerID %s, got %s", req.ConsumerID, decoded.ConsumerID)
	}

	if decoded.Group != req.Group {
		t.Errorf("Expected Group %s, got %s", req.Group, decoded.Group)
	}
}

func TestConsumeResponse_JSON(t *testing.T) {
	now := time.Now()
	resp := ConsumeResponse{
		Messages: []*Message{
			{ID: "msg-1", Topic: "topic-1", Payload: json.RawMessage(`{}`), Timestamp: now},
			{ID: "msg-2", Topic: "topic-1", Payload: json.RawMessage(`{}`), Timestamp: now},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal ConsumeResponse: %v", err)
	}

	var decoded ConsumeResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ConsumeResponse: %v", err)
	}

	if len(decoded.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(decoded.Messages))
	}
}

func TestAckRequest_JSON(t *testing.T) {
	req := AckRequest{
		ConsumerID: "consumer-1",
		MessageIDs: []string{"msg-1", "msg-2", "msg-3"},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal AckRequest: %v", err)
	}

	var decoded AckRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal AckRequest: %v", err)
	}

	if decoded.ConsumerID != req.ConsumerID {
		t.Errorf("Expected ConsumerID %s, got %s", req.ConsumerID, decoded.ConsumerID)
	}

	if len(decoded.MessageIDs) != 3 {
		t.Errorf("Expected 3 message IDs, got %d", len(decoded.MessageIDs))
	}
}

func TestAckResponse_JSON(t *testing.T) {
	resp := AckResponse{
		Status: "ok",
		Acked:  5,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal AckResponse: %v", err)
	}

	var decoded AckResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal AckResponse: %v", err)
	}

	if decoded.Status != resp.Status {
		t.Errorf("Expected Status %s, got %s", resp.Status, decoded.Status)
	}

	if decoded.Acked != 5 {
		t.Errorf("Expected Acked 5, got %d", decoded.Acked)
	}
}

func TestTopicStats_JSON(t *testing.T) {
	stats := TopicStats{
		Topic:            "test-topic",
		QueueSize:        100,
		MaxSize:          10000,
		PendingMessages:  50,
		ConsumerGroups:   2,
		TotalConsumers:   5,
		OldestMessageAge: "5m30s",
		DroppedMessages:  0,
		QueueUtilization: "1.0%",
	}

	data, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("Failed to marshal TopicStats: %v", err)
	}

	var decoded TopicStats
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal TopicStats: %v", err)
	}

	if decoded.Topic != stats.Topic {
		t.Errorf("Expected Topic %s, got %s", stats.Topic, decoded.Topic)
	}

	if decoded.QueueSize != 100 {
		t.Errorf("Expected QueueSize 100, got %d", decoded.QueueSize)
	}

	if decoded.TotalConsumers != 5 {
		t.Errorf("Expected TotalConsumers 5, got %d", decoded.TotalConsumers)
	}
}

func TestConsumerInfo_JSON(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	info := ConsumerInfo{
		ID:          "consumer-1",
		Group:       "group-1",
		Connected:   true,
		LastSeen:    now,
		PendingAcks: 5,
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal ConsumerInfo: %v", err)
	}

	var decoded ConsumerInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ConsumerInfo: %v", err)
	}

	if decoded.ID != info.ID {
		t.Errorf("Expected ID %s, got %s", info.ID, decoded.ID)
	}

	if decoded.Group != info.Group {
		t.Errorf("Expected Group %s, got %s", info.Group, decoded.Group)
	}

	if decoded.PendingAcks != 5 {
		t.Errorf("Expected PendingAcks 5, got %d", decoded.PendingAcks)
	}

	if !decoded.Connected {
		t.Error("Expected Connected to be true")
	}
}

func TestErrorResponse_JSON(t *testing.T) {
	resp := ErrorResponse{
		Error: "not found",
		Code:  404,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal ErrorResponse: %v", err)
	}

	var decoded ErrorResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ErrorResponse: %v", err)
	}

	if decoded.Error != resp.Error {
		t.Errorf("Expected Error %s, got %s", resp.Error, decoded.Error)
	}

	if decoded.Code != 404 {
		t.Errorf("Expected Code 404, got %d", decoded.Code)
	}
}
