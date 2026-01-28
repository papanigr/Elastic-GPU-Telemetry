package models

import "time"

// GPUTelemetry represents a parsed GPU telemetry record.
// This is what will be persisted to the database.
// Matches the streamer's TelemetryRecord structure.
type GPUTelemetry struct {
	ID         string    `json:"id" db:"id"`
	Timestamp  time.Time `json:"timestamp" db:"timestamp"`
	MetricName string    `json:"metric_name" db:"metric_name"`
	GPUIndex   int       `json:"gpu_index" db:"gpu_index"`
	Device     string    `json:"device" db:"device"`
	UUID       string    `json:"uuid" db:"uuid"`
	ModelName  string    `json:"model_name" db:"model_name"`
	Hostname   string    `json:"hostname" db:"hostname"`
	Container  string    `json:"container,omitempty" db:"container"`
	Pod        string    `json:"pod,omitempty" db:"pod"`
	Namespace  string    `json:"namespace,omitempty" db:"namespace"`
	Value      float64   `json:"value" db:"value"`
	LabelsRaw  string    `json:"labels_raw,omitempty" db:"labels_raw"`

	// Metadata added by collector
	ReceivedAt time.Time `json:"received_at" db:"received_at"`
	MessageID  string    `json:"message_id" db:"message_id"`
}

// RawTelemetryPayload is the JSON structure received from the streamer.
// This matches streamer/pkg/models/telemetry.go TelemetryRecord exactly.
type RawTelemetryPayload struct {
	Timestamp  string  `json:"timestamp"`
	MetricName string  `json:"metric_name"`
	GPUIndex   int     `json:"gpu_index"`
	Device     string  `json:"device"`
	UUID       string  `json:"uuid"`
	ModelName  string  `json:"model_name"`
	Hostname   string  `json:"hostname"`
	Container  string  `json:"container,omitempty"`
	Pod        string  `json:"pod,omitempty"`
	Namespace  string  `json:"namespace,omitempty"`
	Value      float64 `json:"value"`
	LabelsRaw  string  `json:"labels_raw,omitempty"`
}
