package repository

import (
	"context"
	"testing"
	"time"

	"github.com/gpu-telemetry-pipeline/gateway/internal/models"
	"github.com/stretchr/testify/assert"
)

// MockRepository is a mock implementation of Repository for testing.
type MockRepository struct {
	GPUs         []models.GPU
	Telemetry    []models.GPUTelemetry
	TotalCount   int
	PingErr      error
	GetGPUsErr   error
	GetGPUErr    error
	GetTelErr    error
	CountTelErr  error
}

func (m *MockRepository) GetGPUs(ctx context.Context) ([]models.GPU, error) {
	if m.GetGPUsErr != nil {
		return nil, m.GetGPUsErr
	}
	return m.GPUs, nil
}

func (m *MockRepository) GetGPUByUUID(ctx context.Context, uuid string) (*models.GPU, error) {
	if m.GetGPUErr != nil {
		return nil, m.GetGPUErr
	}
	for _, gpu := range m.GPUs {
		if gpu.UUID == uuid {
			return &gpu, nil
		}
	}
	return nil, nil
}

func (m *MockRepository) GetTelemetry(ctx context.Context, filter models.TelemetryFilter) ([]models.GPUTelemetry, error) {
	if m.GetTelErr != nil {
		return nil, m.GetTelErr
	}
	return m.Telemetry, nil
}

func (m *MockRepository) CountTelemetry(ctx context.Context, filter models.TelemetryFilter) (int, error) {
	if m.CountTelErr != nil {
		return 0, m.CountTelErr
	}
	return m.TotalCount, nil
}

func (m *MockRepository) Ping(ctx context.Context) error {
	return m.PingErr
}

func (m *MockRepository) Close() error {
	return nil
}

// Test that MockRepository implements Repository interface
var _ Repository = (*MockRepository)(nil)

func TestMockRepository_GetGPUs(t *testing.T) {
	now := time.Now()
	mock := &MockRepository{
		GPUs: []models.GPU{
			{UUID: "GPU-001", GPUIndex: 0, Hostname: "host1", LastSeen: now},
			{UUID: "GPU-002", GPUIndex: 1, Hostname: "host1", LastSeen: now},
		},
	}

	gpus, err := mock.GetGPUs(context.Background())
	assert.NoError(t, err)
	assert.Len(t, gpus, 2)
	assert.Equal(t, "GPU-001", gpus[0].UUID)
}

func TestMockRepository_GetGPUByUUID(t *testing.T) {
	now := time.Now()
	mock := &MockRepository{
		GPUs: []models.GPU{
			{UUID: "GPU-001", GPUIndex: 0, Hostname: "host1", LastSeen: now},
		},
	}

	// Found
	gpu, err := mock.GetGPUByUUID(context.Background(), "GPU-001")
	assert.NoError(t, err)
	assert.NotNil(t, gpu)
	assert.Equal(t, "GPU-001", gpu.UUID)

	// Not found
	gpu, err = mock.GetGPUByUUID(context.Background(), "GPU-999")
	assert.NoError(t, err)
	assert.Nil(t, gpu)
}

func TestMockRepository_GetTelemetry(t *testing.T) {
	now := time.Now()
	mock := &MockRepository{
		Telemetry: []models.GPUTelemetry{
			{ID: "1", UUID: "GPU-001", Timestamp: now, Value: 50.0},
			{ID: "2", UUID: "GPU-001", Timestamp: now.Add(-time.Minute), Value: 45.0},
		},
		TotalCount: 100,
	}

	filter := models.TelemetryFilter{
		GPUUUID: "GPU-001",
		Limit:   10,
		Offset:  0,
	}

	telemetry, err := mock.GetTelemetry(context.Background(), filter)
	assert.NoError(t, err)
	assert.Len(t, telemetry, 2)

	count, err := mock.CountTelemetry(context.Background(), filter)
	assert.NoError(t, err)
	assert.Equal(t, 100, count)
}

func TestBuildTelemetryQuery(t *testing.T) {
	repo := &PostgresRepository{}

	t.Run("basic query", func(t *testing.T) {
		filter := models.TelemetryFilter{
			GPUUUID: "GPU-001",
			Limit:   10,
			Offset:  0,
		}

		query, args := repo.buildTelemetryQuery(filter, false)
		assert.Contains(t, query, "uuid = $1")
		assert.Contains(t, query, "LIMIT $2 OFFSET $3")
		assert.Len(t, args, 3)
		assert.Equal(t, "GPU-001", args[0])
	})

	t.Run("with time filters", func(t *testing.T) {
		start := time.Now().Add(-time.Hour)
		end := time.Now()

		filter := models.TelemetryFilter{
			GPUUUID:   "GPU-001",
			StartTime: &start,
			EndTime:   &end,
			Limit:     10,
			Offset:    0,
		}

		query, args := repo.buildTelemetryQuery(filter, false)
		assert.Contains(t, query, "uuid = $1")
		assert.Contains(t, query, "timestamp >= $2")
		assert.Contains(t, query, "timestamp <= $3")
		assert.Contains(t, query, "LIMIT $4 OFFSET $5")
		assert.Len(t, args, 5)
	})

	t.Run("count query", func(t *testing.T) {
		filter := models.TelemetryFilter{
			GPUUUID: "GPU-001",
		}

		query, args := repo.buildTelemetryQuery(filter, true)
		assert.Contains(t, query, "SELECT COUNT(*)")
		assert.NotContains(t, query, "LIMIT")
		assert.Len(t, args, 1)
	})
}
