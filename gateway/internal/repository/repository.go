package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gpu-telemetry-pipeline/gateway/internal/models"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// Repository defines the interface for telemetry data access.
// This interface enables unit testing with mocks.
type Repository interface {
	// GetGPUs returns all unique GPUs.
	GetGPUs(ctx context.Context) ([]models.GPU, error)

	// GetGPUByUUID returns a specific GPU by UUID.
	GetGPUByUUID(ctx context.Context, uuid string) (*models.GPU, error)

	// GetTelemetry returns telemetry for a GPU with optional filters.
	GetTelemetry(ctx context.Context, filter models.TelemetryFilter) ([]models.GPUTelemetry, error)

	// CountTelemetry returns the total count of telemetry records for a GPU.
	CountTelemetry(ctx context.Context, filter models.TelemetryFilter) (int, error)

	// Ping checks the database connection.
	Ping(ctx context.Context) error

	// Close closes the database connection.
	Close() error
}

// PostgresRepository implements Repository for PostgreSQL.
type PostgresRepository struct {
	db *sqlx.DB
}

// Config holds PostgreSQL connection configuration.
type Config struct {
	ConnectionString string
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
}

// NewPostgresRepository creates a new PostgreSQL repository.
func NewPostgresRepository(cfg Config) (*PostgresRepository, error) {
	db, err := sqlx.Connect("postgres", cfg.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	return &PostgresRepository{db: db}, nil
}

// GetGPUs returns all unique GPUs with their latest info.
func (r *PostgresRepository) GetGPUs(ctx context.Context) ([]models.GPU, error) {
	query := `
		SELECT DISTINCT ON (uuid)
			uuid,
			gpu_index,
			COALESCE(device, '') as device,
			COALESCE(model_name, '') as model_name,
			hostname,
			timestamp as last_seen
		FROM gpu_telemetry
		ORDER BY uuid, timestamp DESC
	`

	var gpus []models.GPU
	if err := r.db.SelectContext(ctx, &gpus, query); err != nil {
		return nil, fmt.Errorf("failed to get GPUs: %w", err)
	}

	return gpus, nil
}

// GetGPUByUUID returns a specific GPU by UUID.
func (r *PostgresRepository) GetGPUByUUID(ctx context.Context, uuid string) (*models.GPU, error) {
	query := `
		SELECT DISTINCT ON (uuid)
			uuid,
			gpu_index,
			COALESCE(device, '') as device,
			COALESCE(model_name, '') as model_name,
			hostname,
			timestamp as last_seen
		FROM gpu_telemetry
		WHERE uuid = $1
		ORDER BY uuid, timestamp DESC
		LIMIT 1
	`

	var gpu models.GPU
	if err := r.db.GetContext(ctx, &gpu, query, uuid); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // GPU not found
		}
		return nil, fmt.Errorf("failed to get GPU: %w", err)
	}

	return &gpu, nil
}

// GetTelemetry returns telemetry for a GPU with optional filters.
func (r *PostgresRepository) GetTelemetry(ctx context.Context, filter models.TelemetryFilter) ([]models.GPUTelemetry, error) {
	query, args := r.buildTelemetryQuery(filter, false)

	var telemetry []models.GPUTelemetry
	if err := r.db.SelectContext(ctx, &telemetry, query, args...); err != nil {
		return nil, fmt.Errorf("failed to get telemetry: %w", err)
	}

	return telemetry, nil
}

// CountTelemetry returns the total count of telemetry records matching the filter.
func (r *PostgresRepository) CountTelemetry(ctx context.Context, filter models.TelemetryFilter) (int, error) {
	query, args := r.buildTelemetryQuery(filter, true)

	var count int
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("failed to count telemetry: %w", err)
	}

	return count, nil
}

// buildTelemetryQuery constructs the SQL query for telemetry with filters.
func (r *PostgresRepository) buildTelemetryQuery(filter models.TelemetryFilter, countOnly bool) (string, []interface{}) {
	var conditions []string
	var args []interface{}
	paramNum := 1

	// UUID filter (required)
	conditions = append(conditions, fmt.Sprintf("uuid = $%d", paramNum))
	args = append(args, filter.GPUUUID)
	paramNum++

	// Start time filter
	if filter.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", paramNum))
		args = append(args, *filter.StartTime)
		paramNum++
	}

	// End time filter
	if filter.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", paramNum))
		args = append(args, *filter.EndTime)
		paramNum++
	}

	whereClause := strings.Join(conditions, " AND ")

	if countOnly {
		return fmt.Sprintf("SELECT COUNT(*) FROM gpu_telemetry WHERE %s", whereClause), args
	}

	// Build full query with pagination
	query := fmt.Sprintf(`
		SELECT
			id,
			timestamp,
			metric_name,
			gpu_index,
			COALESCE(device, '') as device,
			uuid,
			COALESCE(model_name, '') as model_name,
			hostname,
			COALESCE(container, '') as container,
			COALESCE(pod, '') as pod,
			COALESCE(namespace, '') as namespace,
			value,
			COALESCE(labels_raw, '') as labels_raw,
			received_at,
			COALESCE(message_id, '') as message_id
		FROM gpu_telemetry
		WHERE %s
		ORDER BY timestamp DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, paramNum, paramNum+1)

	args = append(args, filter.Limit, filter.Offset)

	return query, args
}

// Ping checks the database connection.
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close closes the database connection.
func (r *PostgresRepository) Close() error {
	return r.db.Close()
}
