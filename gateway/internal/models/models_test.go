package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestGPU_JSON(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	gpu := GPU{
		UUID:      "GPU-123",
		GPUIndex:  0,
		Device:    "nvidia0",
		ModelName: "NVIDIA H100",
		Hostname:  "host1",
		LastSeen:  now,
	}

	data, err := json.Marshal(gpu)
	if err != nil {
		t.Fatalf("Failed to marshal GPU: %v", err)
	}

	var decoded GPU
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal GPU: %v", err)
	}

	if decoded.UUID != gpu.UUID {
		t.Errorf("Expected UUID %s, got %s", gpu.UUID, decoded.UUID)
	}

	if decoded.GPUIndex != gpu.GPUIndex {
		t.Errorf("Expected GPUIndex %d, got %d", gpu.GPUIndex, decoded.GPUIndex)
	}
}

func TestGPUTelemetry_JSON(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	tel := GPUTelemetry{
		ID:         "tel-123",
		Timestamp:  now,
		MetricName: "DCGM_FI_DEV_GPU_UTIL",
		GPUIndex:   0,
		Device:     "nvidia0",
		UUID:       "GPU-123",
		ModelName:  "NVIDIA H100",
		Hostname:   "host1",
		Value:      75.5,
		ReceivedAt: now,
	}

	data, err := json.Marshal(tel)
	if err != nil {
		t.Fatalf("Failed to marshal GPUTelemetry: %v", err)
	}

	var decoded GPUTelemetry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal GPUTelemetry: %v", err)
	}

	if decoded.ID != tel.ID {
		t.Errorf("Expected ID %s, got %s", tel.ID, decoded.ID)
	}

	if decoded.Value != tel.Value {
		t.Errorf("Expected Value %f, got %f", tel.Value, decoded.Value)
	}

	if decoded.MetricName != tel.MetricName {
		t.Errorf("Expected MetricName %s, got %s", tel.MetricName, decoded.MetricName)
	}
}

func TestGPUListResponse_JSON(t *testing.T) {
	now := time.Now()
	resp := GPUListResponse{
		GPUs: []GPU{
			{UUID: "GPU-1", GPUIndex: 0, Hostname: "host1", LastSeen: now},
			{UUID: "GPU-2", GPUIndex: 1, Hostname: "host1", LastSeen: now},
		},
		Count: 2,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal GPUListResponse: %v", err)
	}

	var decoded GPUListResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal GPUListResponse: %v", err)
	}

	if decoded.Count != 2 {
		t.Errorf("Expected Count 2, got %d", decoded.Count)
	}

	if len(decoded.GPUs) != 2 {
		t.Errorf("Expected 2 GPUs, got %d", len(decoded.GPUs))
	}
}

func TestTelemetryListResponse_JSON(t *testing.T) {
	now := time.Now()
	start := now.Add(-time.Hour)
	end := now

	resp := TelemetryListResponse{
		Telemetry: []GPUTelemetry{
			{ID: "1", UUID: "GPU-1", Timestamp: now, Value: 50},
		},
		Count:      1,
		TotalCount: 100,
		GPUUUID:    "GPU-1",
		StartTime:  &start,
		EndTime:    &end,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal TelemetryListResponse: %v", err)
	}

	var decoded TelemetryListResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal TelemetryListResponse: %v", err)
	}

	if decoded.TotalCount != 100 {
		t.Errorf("Expected TotalCount 100, got %d", decoded.TotalCount)
	}

	if decoded.GPUUUID != "GPU-1" {
		t.Errorf("Expected GPUUUID 'GPU-1', got %s", decoded.GPUUUID)
	}
}

func TestErrorResponse_JSON(t *testing.T) {
	resp := ErrorResponse{
		Error:   "Not found",
		Code:    404,
		Details: "GPU-123 not found",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal ErrorResponse: %v", err)
	}

	var decoded ErrorResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ErrorResponse: %v", err)
	}

	if decoded.Code != 404 {
		t.Errorf("Expected Code 404, got %d", decoded.Code)
	}

	if decoded.Error != "Not found" {
		t.Errorf("Expected Error 'Not found', got %s", decoded.Error)
	}
}

func TestHealthResponse_JSON(t *testing.T) {
	resp := HealthResponse{
		Status:   "healthy",
		Database: "healthy",
		Version:  "1.0.0",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal HealthResponse: %v", err)
	}

	var decoded HealthResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal HealthResponse: %v", err)
	}

	if decoded.Status != "healthy" {
		t.Errorf("Expected Status 'healthy', got %s", decoded.Status)
	}

	if decoded.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", decoded.Version)
	}
}

func TestTelemetryFilter(t *testing.T) {
	start := time.Now().Add(-time.Hour)
	end := time.Now()

	filter := TelemetryFilter{
		GPUUUID:   "GPU-123",
		StartTime: &start,
		EndTime:   &end,
		Limit:     50,
		Offset:    10,
	}

	if filter.GPUUUID != "GPU-123" {
		t.Errorf("Expected GPUUUID 'GPU-123', got %s", filter.GPUUUID)
	}

	if filter.Limit != 50 {
		t.Errorf("Expected Limit 50, got %d", filter.Limit)
	}

	if filter.Offset != 10 {
		t.Errorf("Expected Offset 10, got %d", filter.Offset)
	}
}
