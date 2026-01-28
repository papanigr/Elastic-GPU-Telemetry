package internal

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gpu-telemetry-pipeline/streamer/pkg/models"
	"github.com/rs/zerolog"
)

// CSVReader reads telemetry records from a CSV file.
type CSVReader struct {
	filePath   string
	file       *os.File
	reader     *csv.Reader
	headers    []string
	headerMap  map[string]int
	lineNumber int
	mu         sync.Mutex
	logger     zerolog.Logger
}

// NewCSVReader creates a new CSVReader for the given file path.
func NewCSVReader(filePath string, logger zerolog.Logger) (*CSVReader, error) {
	r := &CSVReader{
		filePath:  filePath,
		headerMap: make(map[string]int),
		logger:    logger.With().Str("component", "csv-reader").Logger(),
	}

	if err := r.open(); err != nil {
		return nil, err
	}

	return r, nil
}

// open opens the CSV file and reads the headers.
func (r *CSVReader) open() error {
	file, err := os.Open(r.filePath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	r.file = file
	r.reader = csv.NewReader(file)
	r.lineNumber = 0

	// Read headers
	headers, err := r.reader.Read()
	if err != nil {
		r.file.Close()
		return fmt.Errorf("failed to read CSV headers: %w", err)
	}
	r.headers = headers
	r.lineNumber++

	// Build header map for quick lookup
	for i, header := range headers {
		r.headerMap[header] = i
	}

	r.logger.Info().
		Str("file", r.filePath).
		Int("header_count", len(headers)).
		Msg("CSV file opened")

	return nil
}

// ReadNext reads the next telemetry record from the CSV file.
// When EOF is reached, it resets to the beginning of the file.
func (r *CSVReader) ReadNext() (*models.TelemetryRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, err := r.reader.Read()
	if err == io.EOF {
		// Reset to beginning of file
		r.logger.Debug().Msg("Reached EOF, resetting to beginning")
		if err := r.reset(); err != nil {
			return nil, fmt.Errorf("failed to reset CSV reader: %w", err)
		}
		// Read the first record after headers
		record, err = r.reader.Read()
		if err != nil {
			return nil, fmt.Errorf("failed to read after reset: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to read CSV record: %w", err)
	}

	r.lineNumber++

	// Parse the record
	telemetry, err := r.parseRecord(record)
	if err != nil {
		return nil, fmt.Errorf("failed to parse record at line %d: %w", r.lineNumber, err)
	}

	return telemetry, nil
}

// reset resets the reader to the beginning of the file.
func (r *CSVReader) reset() error {
	// Close current file
	if r.file != nil {
		r.file.Close()
	}

	// Reopen the file
	return r.open()
}

// parseRecord parses a CSV record into a TelemetryRecord.
func (r *CSVReader) parseRecord(record []string) (*models.TelemetryRecord, error) {
	telemetry := &models.TelemetryRecord{
		// Use current time as the timestamp
		Timestamp: time.Now(),
	}

	// Helper function to get field by header name
	getField := func(name string) string {
		if idx, ok := r.headerMap[name]; ok && idx < len(record) {
			return strings.Trim(record[idx], "\"")
		}
		return ""
	}

	// Parse fields
	telemetry.MetricName = getField("metric_name")
	telemetry.Device = getField("device")
	telemetry.UUID = getField("uuid")
	telemetry.ModelName = getField("modelName")
	telemetry.Hostname = getField("Hostname")
	telemetry.Container = getField("container")
	telemetry.Pod = getField("pod")
	telemetry.Namespace = getField("namespace")
	telemetry.LabelsRaw = getField("labels_raw")

	// Parse gpu_id as integer
	gpuIDStr := getField("gpu_id")
	if gpuIDStr != "" {
		gpuID, err := strconv.Atoi(gpuIDStr)
		if err != nil {
			r.logger.Warn().
				Str("gpu_id", gpuIDStr).
				Err(err).
				Msg("Failed to parse gpu_id, using 0")
			telemetry.GPUIndex = 0
		} else {
			telemetry.GPUIndex = gpuID
		}
	}

	// Parse value as float64
	valueStr := getField("value")
	if valueStr != "" {
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			r.logger.Warn().
				Str("value", valueStr).
				Err(err).
				Msg("Failed to parse value, using 0")
			telemetry.Value = 0
		} else {
			telemetry.Value = value
		}
	}

	return telemetry, nil
}

// Close closes the CSV file.
func (r *CSVReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// GetLineNumber returns the current line number.
func (r *CSVReader) GetLineNumber() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lineNumber
}

// GetHeaders returns the CSV headers.
func (r *CSVReader) GetHeaders() []string {
	return r.headers
}

// ReadAll reads all records from the CSV file and resets to the beginning.
// Each record gets the current timestamp.
func (r *CSVReader) ReadAll() ([]*models.TelemetryRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var records []*models.TelemetryRecord

	for {
		record, err := r.reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV record at line %d: %w", r.lineNumber, err)
		}
		r.lineNumber++

		telemetry, err := r.parseRecord(record)
		if err != nil {
			r.logger.Warn().
				Int("line", r.lineNumber).
				Err(err).
				Msg("Failed to parse record, skipping")
			continue
		}
		records = append(records, telemetry)
	}

	// Reset to beginning for next batch
	if err := r.resetInternal(); err != nil {
		return nil, fmt.Errorf("failed to reset after reading all: %w", err)
	}

	r.logger.Debug().
		Int("record_count", len(records)).
		Msg("Read all records from CSV")

	return records, nil
}

// resetInternal resets the reader without locking (for internal use).
func (r *CSVReader) resetInternal() error {
	if r.file != nil {
		r.file.Close()
	}

	file, err := os.Open(r.filePath)
	if err != nil {
		return fmt.Errorf("failed to reopen CSV file: %w", err)
	}
	r.file = file
	r.reader = csv.NewReader(file)
	r.lineNumber = 0

	// Skip headers
	_, err = r.reader.Read()
	if err != nil {
		r.file.Close()
		return fmt.Errorf("failed to read CSV headers: %w", err)
	}
	r.lineNumber++

	return nil
}
