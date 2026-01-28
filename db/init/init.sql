-- PostgreSQL initialization script
-- This script runs when the database container starts for the first time

-- Create the telemetry database if it doesn't exist
-- (handled by POSTGRES_DB env var, but keeping for reference)

-- Run migrations in order
\i /docker-entrypoint-initdb.d/migrations/001_create_gpu_telemetry.sql
\i /docker-entrypoint-initdb.d/migrations/002_create_gpus_view.sql

-- Grant permissions to the postgres user
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO postgres;

-- Log completion
DO $$
BEGIN
    RAISE NOTICE 'Database initialization complete!';
END $$;
