package models

import (
	"testing"
	"time"
)

func TestGPUTelemetry(t *testing.T) {
	now := time.Now()
	tel := GPUTelemetry{
		ID:         "test-id",
		Timestamp:  now,
		MetricName: "DCGM_FI_DEV_GPU_UTIL",
		GPUIndex:   0,
		Device:     "nvidia0",
		UUID:       "GPU-123",
		ModelName:  "NVIDIA H100",
		Hostname:   "host1",
		Container:  "container1",
		Pod:        "pod1",
		Namespace:  "default",
		Value:      75.5,
		LabelsRaw:  "label1=value1",
		ReceivedAt: now,
		MessageID:  "msg-123",
	}

	if tel.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", tel.ID)
	}

	if tel.MetricName != "DCGM_FI_DEV_GPU_UTIL" {
		t.Errorf("Expected MetricName 'DCGM_FI_DEV_GPU_UTIL', got %s", tel.MetricName)
	}

	if tel.Value != 75.5 {
		t.Errorf("Expected Value 75.5, got %f", tel.Value)
	}

	if tel.UUID != "GPU-123" {
		t.Errorf("Expected UUID 'GPU-123', got %s", tel.UUID)
	}

	if tel.GPUIndex != 0 {
		t.Errorf("Expected GPUIndex 0, got %d", tel.GPUIndex)
	}
}

func TestRawTelemetryPayload(t *testing.T) {
	raw := RawTelemetryPayload{
		Timestamp:  "2026-01-28T15:30:00Z",
		MetricName: "DCGM_FI_DEV_GPU_UTIL",
		GPUIndex:   1,
		Device:     "nvidia1",
		UUID:       "GPU-456",
		ModelName:  "NVIDIA A100",
		Hostname:   "host2",
		Container:  "",
		Pod:        "",
		Namespace:  "",
		Value:      100.0,
		LabelsRaw:  "",
	}

	if raw.MetricName != "DCGM_FI_DEV_GPU_UTIL" {
		t.Errorf("Expected MetricName 'DCGM_FI_DEV_GPU_UTIL', got %s", raw.MetricName)
	}

	if raw.UUID != "GPU-456" {
		t.Errorf("Expected UUID 'GPU-456', got %s", raw.UUID)
	}

	if raw.Value != 100.0 {
		t.Errorf("Expected Value 100.0, got %f", raw.Value)
	}

	if raw.GPUIndex != 1 {
		t.Errorf("Expected GPUIndex 1, got %d", raw.GPUIndex)
	}
}

func TestRawTelemetryPayload_EmptyOptionalFields(t *testing.T) {
	// Kubernetes fields are optional
	raw := RawTelemetryPayload{
		Timestamp:  "2026-01-28T15:30:00Z",
		MetricName: "GPU_UTIL",
		GPUIndex:   0,
		UUID:       "GPU-789",
		Hostname:   "host3",
		Value:      50.0,
	}

	if raw.Container != "" {
		t.Errorf("Expected empty Container, got %s", raw.Container)
	}

	if raw.Pod != "" {
		t.Errorf("Expected empty Pod, got %s", raw.Pod)
	}

	if raw.Namespace != "" {
		t.Errorf("Expected empty Namespace, got %s", raw.Namespace)
	}
}
