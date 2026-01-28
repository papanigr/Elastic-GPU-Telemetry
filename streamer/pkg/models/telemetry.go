// Package models contains shared data models used across the application.
package models

import (
	"time"
)

// TelemetryRecord represents a single GPU telemetry data point.
// This matches the DCGM metrics CSV format with an updated timestamp.
type TelemetryRecord struct {
	// Timestamp is the time when this record was processed (current time, not from CSV)
	Timestamp time.Time `json:"timestamp"`

	// MetricName is the DCGM metric name (e.g., DCGM_FI_DEV_GPU_UTIL)
	MetricName string `json:"metric_name"`

	// GPUIndex is the GPU index on the host (0, 1, 2, etc.)
	GPUIndex int `json:"gpu_index"`

	// Device is the device name (e.g., nvidia0)
	Device string `json:"device"`

	// UUID is the unique identifier for the GPU
	UUID string `json:"uuid"`

	// ModelName is the GPU model (e.g., NVIDIA H100 80GB HBM3)
	ModelName string `json:"model_name"`

	// Hostname is the host where the GPU is located
	Hostname string `json:"hostname"`

	// Container is the container name (if applicable)
	Container string `json:"container,omitempty"`

	// Pod is the Kubernetes pod name (if applicable)
	Pod string `json:"pod,omitempty"`

	// Namespace is the Kubernetes namespace (if applicable)
	Namespace string `json:"namespace,omitempty"`

	// Value is the metric value
	Value float64 `json:"value"`

	// LabelsRaw contains the raw labels string from DCGM
	LabelsRaw string `json:"labels_raw,omitempty"`
}

// GPU represents a unique GPU device.
type GPU struct {
	UUID      string `json:"uuid"`
	Index     int    `json:"index"`
	Device    string `json:"device"`
	ModelName string `json:"model_name"`
	Hostname  string `json:"hostname"`
}

// Message represents a message to be sent to the message queue.
type Message struct {
	ID        string          `json:"id"`
	Topic     string          `json:"topic"`
	Payload   TelemetryRecord `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
}

// PublishRequest represents a request to publish a message to the MQ.
type PublishRequest struct {
	Payload []byte `json:"payload"`
}

// PublishResponse represents the response from publishing a message.
type PublishResponse struct {
	MessageID string    `json:"message_id"`
	Topic     string    `json:"topic"`
	Timestamp time.Time `json:"timestamp"`
}
