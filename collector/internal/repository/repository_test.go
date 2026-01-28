package repository

import (
	"context"
	"testing"
	"time"

	"github.com/gpu-telemetry-pipeline/collector/internal/models"
	"github.com/rs/zerolog"
)

func TestNoOpRepository_Save(t *testing.T) {
	logger := zerolog.Nop()
	repo := NewNoOpRepository(logger)

	records := []models.GPUTelemetry{
		{
			ID:         "1",
			Timestamp:  time.Now(),
			MetricName: "GPU_UTIL",
			GPUIndex:   0,
			UUID:       "GPU-123",
			Hostname:   "host1",
			Value:      50.0,
		},
		{
			ID:         "2",
			Timestamp:  time.Now(),
			MetricName: "GPU_UTIL",
			GPUIndex:   1,
			UUID:       "GPU-456",
			Hostname:   "host1",
			Value:      75.0,
		},
	}

	ctx := context.Background()
	err := repo.Save(ctx, records)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestNoOpRepository_SaveEmpty(t *testing.T) {
	logger := zerolog.Nop()
	repo := NewNoOpRepository(logger)

	ctx := context.Background()
	err := repo.Save(ctx, []models.GPUTelemetry{})

	if err != nil {
		t.Errorf("Unexpected error for empty records: %v", err)
	}
}

func TestNoOpRepository_Stats(t *testing.T) {
	logger := zerolog.Nop()
	repo := NewNoOpRepository(logger)

	stats := repo.Stats()

	// Just verify stats are returned
	if stats.RecordsSaved < 0 {
		t.Error("RecordsSaved should not be negative")
	}

	if stats.ErrorCount < 0 {
		t.Error("ErrorCount should not be negative")
	}
}

func TestNoOpRepository_Close(t *testing.T) {
	logger := zerolog.Nop()
	repo := NewNoOpRepository(logger)

	err := repo.Close()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRepository_Interface(t *testing.T) {
	// Verify NoOpRepository implements Repository interface
	logger := zerolog.Nop()
	var repo Repository = NewNoOpRepository(logger)

	if repo == nil {
		t.Fatal("Expected repository to be created")
	}
}

func TestNoOpRepository_MultipleSaves(t *testing.T) {
	logger := zerolog.Nop()
	repo := NewNoOpRepository(logger)

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		records := []models.GPUTelemetry{
			{
				ID:         string(rune('a' + i)),
				Timestamp:  time.Now(),
				MetricName: "GPU_TEMP",
				GPUIndex:   i,
				UUID:       "GPU-" + string(rune('0'+i)),
				Hostname:   "host1",
				Value:      float64(50 + i),
			},
		}
		err := repo.Save(ctx, records)
		if err != nil {
			t.Errorf("Unexpected error on save %d: %v", i, err)
		}
	}
}

func TestNoOpRepository_ContextCancellation(t *testing.T) {
	logger := zerolog.Nop()
	repo := NewNoOpRepository(logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	records := []models.GPUTelemetry{
		{ID: "1", Timestamp: time.Now(), UUID: "GPU-123"},
	}

	// NoOpRepository should still work even with cancelled context
	// as it doesn't actually do any I/O
	err := repo.Save(ctx, records)
	if err != nil {
		t.Errorf("NoOpRepository should not fail on cancelled context: %v", err)
	}
}
