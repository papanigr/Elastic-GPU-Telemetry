-- Migration: 002_create_gpus_view
-- Description: Create a view for unique GPUs (for GET /api/v1/gpus endpoint)
-- Created: 2026-01-28

-- Create view for unique GPUs with latest info
CREATE OR REPLACE VIEW gpus AS
SELECT DISTINCT ON (uuid)
    uuid,
    gpu_index,
    device,
    model_name,
    hostname,
    MAX(timestamp) OVER (PARTITION BY uuid) as last_seen
FROM gpu_telemetry
ORDER BY uuid, timestamp DESC;

-- Create materialized view for better performance (refresh periodically)
CREATE MATERIALIZED VIEW IF NOT EXISTS gpus_summary AS
SELECT 
    uuid,
    gpu_index,
    device,
    model_name,
    hostname,
    COUNT(*) as record_count,
    MIN(timestamp) as first_seen,
    MAX(timestamp) as last_seen
FROM gpu_telemetry
GROUP BY uuid, gpu_index, device, model_name, hostname;

-- Create index on materialized view
CREATE UNIQUE INDEX IF NOT EXISTS idx_gpus_summary_uuid 
    ON gpus_summary(uuid);

-- Add comment
COMMENT ON MATERIALIZED VIEW gpus_summary IS 'Summary of unique GPUs for API listing';
