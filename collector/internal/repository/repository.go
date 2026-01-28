package repository

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gpu-telemetry-pipeline/collector/internal/models"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/rs/zerolog"
)

// Repository defines the interface for telemetry persistence.
type Repository interface {
	// Save persists a batch of telemetry records.
	Save(ctx context.Context, records []models.GPUTelemetry) error

	// Close closes the repository connection.
	Close() error

	// Stats returns repository statistics.
	Stats() Stats
}

// Stats holds repository statistics.
type Stats struct {
	RecordsSaved int64
	BatchesSaved int64
	LastSaveTime time.Time
	ErrorCount   int64
}

// NoOpRepository is a repository that prints records instead of persisting.
// Used for testing and dry-run mode.
type NoOpRepository struct {
	logger       zerolog.Logger
	recordsSaved atomic.Int64
	batchesSaved atomic.Int64
	lastSaveTime atomic.Value
	errorCount   atomic.Int64
}

// NewNoOpRepository creates a new NoOp repository.
func NewNoOpRepository(logger zerolog.Logger) *NoOpRepository {
	r := &NoOpRepository{
		logger: logger.With().Str("component", "noop-repository").Logger(),
	}
	r.lastSaveTime.Store(time.Time{})
	return r
}

// Save prints the records that would be saved to the database.
func (r *NoOpRepository) Save(ctx context.Context, records []models.GPUTelemetry) error {
	r.logger.Info().
		Int("count", len(records)).
		Msg("=== Records to be saved to PostgreSQL ===")

	for i, record := range records {
		// Log all fields that will be stored in PostgreSQL
		r.logger.Info().
			Int("index", i+1).
			Time("timestamp", record.Timestamp).
			Str("metric_name", record.MetricName).
			Int("gpu_index", record.GPUIndex).
			Str("device", record.Device).
			Str("uuid", record.UUID).
			Str("model_name", record.ModelName).
			Str("hostname", record.Hostname).
			Str("container", record.Container).
			Str("pod", record.Pod).
			Str("namespace", record.Namespace).
			Float64("value", record.Value).
			Str("labels_raw", record.LabelsRaw).
			Msg("GPU Telemetry Record")
	}

	r.logger.Info().
		Int("count", len(records)).
		Msg("=== End of batch ===")

	r.recordsSaved.Add(int64(len(records)))
	r.batchesSaved.Add(1)
	r.lastSaveTime.Store(time.Now())

	return nil
}

// Close is a no-op for the NoOp repository.
func (r *NoOpRepository) Close() error {
	return nil
}

// Stats returns repository statistics.
func (r *NoOpRepository) Stats() Stats {
	lastSave, _ := r.lastSaveTime.Load().(time.Time)
	return Stats{
		RecordsSaved: r.recordsSaved.Load(),
		BatchesSaved: r.batchesSaved.Load(),
		LastSaveTime: lastSave,
		ErrorCount:   r.errorCount.Load(),
	}
}

// PostgresRepository implements Repository for PostgreSQL.
type PostgresRepository struct {
	db           *sqlx.DB
	logger       zerolog.Logger
	recordsSaved atomic.Int64
	batchesSaved atomic.Int64
	lastSaveTime atomic.Value
	errorCount   atomic.Int64
}

// PostgresConfig holds PostgreSQL connection configuration.
type PostgresConfig struct {
	ConnectionString string
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
	ConnMaxIdleTime  time.Duration
}

// DefaultPostgresConfig returns default PostgreSQL configuration.
func DefaultPostgresConfig(connString string) PostgresConfig {
	return PostgresConfig{
		ConnectionString: connString,
		MaxOpenConns:     25,
		MaxIdleConns:     5,
		ConnMaxLifetime:  5 * time.Minute,
		ConnMaxIdleTime:  1 * time.Minute,
	}
}

// NewPostgresRepository creates a new PostgreSQL repository.
func NewPostgresRepository(cfg PostgresConfig, logger zerolog.Logger) (*PostgresRepository, error) {
	log := logger.With().Str("component", "postgres-repository").Logger()

	log.Info().Msg("Connecting to PostgreSQL...")

	db, err := sqlx.Connect("postgres", cfg.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	log.Info().
		Int("max_open_conns", cfg.MaxOpenConns).
		Int("max_idle_conns", cfg.MaxIdleConns).
		Msg("Connected to PostgreSQL")

	repo := &PostgresRepository{
		db:     db,
		logger: log,
	}
	repo.lastSaveTime.Store(time.Time{})

	return repo, nil
}

// Save persists telemetry records to PostgreSQL using batch insert.
func (r *PostgresRepository) Save(ctx context.Context, records []models.GPUTelemetry) error {
	if len(records) == 0 {
		return nil
	}

	// Build batch insert query
	query, args := r.buildBatchInsert(records)

	// Execute insert
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.errorCount.Add(1)
		r.logger.Error().
			Err(err).
			Int("batch_size", len(records)).
			Msg("Failed to insert records")
		return fmt.Errorf("failed to insert records: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	r.recordsSaved.Add(rowsAffected)
	r.batchesSaved.Add(1)
	r.lastSaveTime.Store(time.Now())

	r.logger.Debug().
		Int64("rows_inserted", rowsAffected).
		Int("batch_size", len(records)).
		Msg("Records inserted successfully")

	return nil
}

// buildBatchInsert creates a batch INSERT statement with all values.
func (r *PostgresRepository) buildBatchInsert(records []models.GPUTelemetry) (string, []interface{}) {
	// Column names
	columns := []string{
		"id", "timestamp", "metric_name", "gpu_index", "device",
		"uuid", "model_name", "hostname", "container", "pod",
		"namespace", "value", "labels_raw", "received_at", "message_id",
	}

	// Build placeholders: ($1, $2, ...), ($16, $17, ...), ...
	var placeholders []string
	var args []interface{}
	paramNum := 1

	for _, rec := range records {
		var rowPlaceholders []string
		for range columns {
			rowPlaceholders = append(rowPlaceholders, fmt.Sprintf("$%d", paramNum))
			paramNum++
		}
		placeholders = append(placeholders, "("+strings.Join(rowPlaceholders, ", ")+")")

		// Add values in column order
		args = append(args,
			rec.ID,
			rec.Timestamp,
			rec.MetricName,
			rec.GPUIndex,
			rec.Device,
			rec.UUID,
			rec.ModelName,
			rec.Hostname,
			rec.Container,
			rec.Pod,
			rec.Namespace,
			rec.Value,
			rec.LabelsRaw,
			rec.ReceivedAt,
			rec.MessageID,
		)
	}

	query := fmt.Sprintf(
		"INSERT INTO gpu_telemetry (%s) VALUES %s ON CONFLICT (id) DO NOTHING",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	return query, args
}

// Close closes the database connection.
func (r *PostgresRepository) Close() error {
	r.logger.Info().Msg("Closing PostgreSQL connection")
	return r.db.Close()
}

// Stats returns repository statistics.
func (r *PostgresRepository) Stats() Stats {
	lastSave, _ := r.lastSaveTime.Load().(time.Time)
	return Stats{
		RecordsSaved: r.recordsSaved.Load(),
		BatchesSaved: r.batchesSaved.Load(),
		LastSaveTime: lastSave,
		ErrorCount:   r.errorCount.Load(),
	}
}

// Ping checks the database connection.
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}
