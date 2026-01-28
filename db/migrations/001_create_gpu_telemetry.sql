-- Migration: 001_create_gpu_telemetry
-- Description: Create the gpu_telemetry table for storing GPU metrics
-- Created: 2026-01-28

-- Create the gpu_telemetry table
CREATE TABLE IF NOT EXISTS gpu_telemetry (
    id               VARCHAR(36)      PRIMARY KEY,
    timestamp        TIMESTAMPTZ      NOT NULL,
    metric_name      VARCHAR(64)      NOT NULL,
    gpu_index        INTEGER          NOT NULL,
    device           VARCHAR(32),
    uuid             VARCHAR(64)      NOT NULL,
    model_name       VARCHAR(128),
    hostname         VARCHAR(128)     NOT NULL,
    container        VARCHAR(128),
    pod              VARCHAR(128),
    namespace        VARCHAR(64),
    value            DOUBLE PRECISION NOT NULL,
    labels_raw       TEXT,
    received_at      TIMESTAMPTZ      DEFAULT NOW(),
    message_id       VARCHAR(36)
);

-- Create indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_gpu_telemetry_timestamp 
    ON gpu_telemetry(timestamp);

CREATE INDEX IF NOT EXISTS idx_gpu_telemetry_uuid 
    ON gpu_telemetry(uuid);

CREATE INDEX IF NOT EXISTS idx_gpu_telemetry_uuid_timestamp 
    ON gpu_telemetry(uuid, timestamp);

CREATE INDEX IF NOT EXISTS idx_gpu_telemetry_hostname 
    ON gpu_telemetry(hostname);

CREATE INDEX IF NOT EXISTS idx_gpu_telemetry_metric_name 
    ON gpu_telemetry(metric_name);

-- Add comments for documentation
COMMENT ON TABLE gpu_telemetry IS 'GPU telemetry data from DCGM exporter';
COMMENT ON COLUMN gpu_telemetry.id IS 'Unique record identifier (from message ID)';
COMMENT ON COLUMN gpu_telemetry.timestamp IS 'Time when the metric was collected';
COMMENT ON COLUMN gpu_telemetry.metric_name IS 'DCGM metric name (e.g., DCGM_FI_DEV_GPU_UTIL)';
COMMENT ON COLUMN gpu_telemetry.gpu_index IS 'GPU index on the host (0-7)';
COMMENT ON COLUMN gpu_telemetry.device IS 'Linux device name (e.g., nvidia0)';
COMMENT ON COLUMN gpu_telemetry.uuid IS 'Unique GPU identifier';
COMMENT ON COLUMN gpu_telemetry.model_name IS 'GPU model (e.g., NVIDIA H100 80GB HBM3)';
COMMENT ON COLUMN gpu_telemetry.hostname IS 'Host where the GPU is located';
COMMENT ON COLUMN gpu_telemetry.container IS 'Container name (if running in container)';
COMMENT ON COLUMN gpu_telemetry.pod IS 'Kubernetes pod name (if running in K8s)';
COMMENT ON COLUMN gpu_telemetry.namespace IS 'Kubernetes namespace (if running in K8s)';
COMMENT ON COLUMN gpu_telemetry.value IS 'Metric value';
COMMENT ON COLUMN gpu_telemetry.labels_raw IS 'Raw Prometheus labels from DCGM';
COMMENT ON COLUMN gpu_telemetry.received_at IS 'Time when the record was received by collector';
COMMENT ON COLUMN gpu_telemetry.message_id IS 'Message queue message ID for tracing';
