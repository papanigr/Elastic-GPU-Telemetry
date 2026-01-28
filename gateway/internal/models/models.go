package models

import "time"

// GPU represents a unique GPU in the system.
// @Description GPU information
type GPU struct {
	UUID      string    `json:"uuid" db:"uuid" example:"GPU-5fd4f087-86f3-7a43-b711-4771313afc50"`
	GPUIndex  int       `json:"gpu_index" db:"gpu_index" example:"0"`
	Device    string    `json:"device,omitempty" db:"device" example:"nvidia0"`
	ModelName string    `json:"model_name,omitempty" db:"model_name" example:"NVIDIA H100 80GB HBM3"`
	Hostname  string    `json:"hostname" db:"hostname" example:"mtv5-dgx1-hgpu-031"`
	LastSeen  time.Time `json:"last_seen" db:"last_seen" example:"2026-01-28T15:30:00Z"`
}

// GPUTelemetry represents a single telemetry record.
// @Description GPU telemetry data point
type GPUTelemetry struct {
	ID         string    `json:"id" db:"id" example:"abc123-def456"`
	Timestamp  time.Time `json:"timestamp" db:"timestamp" example:"2026-01-28T15:30:00Z"`
	MetricName string    `json:"metric_name" db:"metric_name" example:"DCGM_FI_DEV_GPU_UTIL"`
	GPUIndex   int       `json:"gpu_index" db:"gpu_index" example:"0"`
	Device     string    `json:"device,omitempty" db:"device" example:"nvidia0"`
	UUID       string    `json:"uuid" db:"uuid" example:"GPU-5fd4f087-86f3-7a43-b711-4771313afc50"`
	ModelName  string    `json:"model_name,omitempty" db:"model_name" example:"NVIDIA H100 80GB HBM3"`
	Hostname   string    `json:"hostname" db:"hostname" example:"mtv5-dgx1-hgpu-031"`
	Container  string    `json:"container,omitempty" db:"container"`
	Pod        string    `json:"pod,omitempty" db:"pod"`
	Namespace  string    `json:"namespace,omitempty" db:"namespace"`
	Value      float64   `json:"value" db:"value" example:"75.5"`
	LabelsRaw  string    `json:"labels_raw,omitempty" db:"labels_raw"`
	ReceivedAt time.Time `json:"received_at" db:"received_at" example:"2026-01-28T15:30:01Z"`
	MessageID  string    `json:"message_id,omitempty" db:"message_id"`
}

// TelemetryFilter holds query parameters for filtering telemetry.
type TelemetryFilter struct {
	GPUUUID   string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}

// GPUListResponse is the response for GET /api/v1/gpus.
// @Description List of GPUs response
type GPUListResponse struct {
	GPUs  []GPU `json:"gpus"`
	Count int   `json:"count" example:"8"`
}

// TelemetryListResponse is the response for GET /api/v1/gpus/{id}/telemetry.
// @Description GPU telemetry response with pagination
type TelemetryListResponse struct {
	Telemetry  []GPUTelemetry `json:"telemetry"`
	Count      int            `json:"count" example:"100"`
	TotalCount int            `json:"total_count,omitempty" example:"1000"`
	GPUUUID    string         `json:"gpu_uuid" example:"GPU-5fd4f087-86f3-7a43-b711-4771313afc50"`
	StartTime  *time.Time     `json:"start_time,omitempty"`
	EndTime    *time.Time     `json:"end_time,omitempty"`
}

// ErrorResponse represents an API error.
// @Description API error response
type ErrorResponse struct {
	Error   string `json:"error" example:"GPU not found"`
	Code    int    `json:"code" example:"404"`
	Details string `json:"details,omitempty" example:"GPU-invalid-uuid"`
}

// HealthResponse represents health check response.
// @Description Health check response
type HealthResponse struct {
	Status   string `json:"status" example:"healthy"`
	Database string `json:"database" example:"healthy"`
	Version  string `json:"version" example:"1.0.0"`
}
