package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gpu-telemetry-pipeline/gateway/internal/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository for testing handlers
type mockRepository struct {
	gpus       []models.GPU
	telemetry  []models.GPUTelemetry
	totalCount int
	pingErr    error
	getGPUsErr error
	getGPUErr  error
	getTelErr  error
	countErr   error
}

func (m *mockRepository) GetGPUs(ctx context.Context) ([]models.GPU, error) {
	if m.getGPUsErr != nil {
		return nil, m.getGPUsErr
	}
	return m.gpus, nil
}

func (m *mockRepository) GetGPUByUUID(ctx context.Context, uuid string) (*models.GPU, error) {
	if m.getGPUErr != nil {
		return nil, m.getGPUErr
	}
	for _, gpu := range m.gpus {
		if gpu.UUID == uuid {
			return &gpu, nil
		}
	}
	return nil, nil
}

func (m *mockRepository) GetTelemetry(ctx context.Context, filter models.TelemetryFilter) ([]models.GPUTelemetry, error) {
	if m.getTelErr != nil {
		return nil, m.getTelErr
	}
	return m.telemetry, nil
}

func (m *mockRepository) CountTelemetry(ctx context.Context, filter models.TelemetryFilter) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.totalCount, nil
}

func (m *mockRepository) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *mockRepository) Close() error {
	return nil
}

func newTestHandler(repo *mockRepository) *Handler {
	logger := zerolog.Nop()
	return NewHandler(repo, logger, 100, 1000)
}

func TestHealth(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		repo := &mockRepository{}
		handler := newTestHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		handler.Health(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.HealthResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "healthy", resp.Status)
		assert.Equal(t, "healthy", resp.Database)
		assert.Equal(t, Version, resp.Version)
	})

	t.Run("degraded", func(t *testing.T) {
		repo := &mockRepository{pingErr: errors.New("connection refused")}
		handler := newTestHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		handler.Health(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.HealthResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "degraded", resp.Status)
		assert.Equal(t, "unhealthy", resp.Database)
	})
}

func TestGetGPUs(t *testing.T) {
	t.Run("success with GPUs", func(t *testing.T) {
		now := time.Now()
		repo := &mockRepository{
			gpus: []models.GPU{
				{UUID: "GPU-001", GPUIndex: 0, Hostname: "host1", ModelName: "H100", LastSeen: now},
				{UUID: "GPU-002", GPUIndex: 1, Hostname: "host1", ModelName: "H100", LastSeen: now},
			},
		}
		handler := newTestHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus", nil)
		w := httptest.NewRecorder()

		handler.GetGPUs(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.GPUListResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.GPUs, 2)
		assert.Equal(t, 2, resp.Count)
	})

	t.Run("success empty", func(t *testing.T) {
		repo := &mockRepository{gpus: []models.GPU{}}
		handler := newTestHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus", nil)
		w := httptest.NewRecorder()

		handler.GetGPUs(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.GPUListResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.GPUs, 0)
		assert.Equal(t, 0, resp.Count)
	})

	t.Run("database error", func(t *testing.T) {
		repo := &mockRepository{getGPUsErr: errors.New("database error")}
		handler := newTestHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus", nil)
		w := httptest.NewRecorder()

		handler.GetGPUs(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var resp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Error, "Failed")
	})
}

func TestGetGPUTelemetry(t *testing.T) {
	now := time.Now()
	testGPU := models.GPU{UUID: "GPU-001", GPUIndex: 0, Hostname: "host1", LastSeen: now}
	testTelemetry := []models.GPUTelemetry{
		{ID: "1", UUID: "GPU-001", Timestamp: now, Value: 50.0, MetricName: "GPU_UTIL"},
		{ID: "2", UUID: "GPU-001", Timestamp: now.Add(-time.Minute), Value: 45.0, MetricName: "GPU_UTIL"},
	}

	t.Run("success", func(t *testing.T) {
		repo := &mockRepository{
			gpus:       []models.GPU{testGPU},
			telemetry:  testTelemetry,
			totalCount: 100,
		}
		handler := newTestHandler(repo)

		// Create request with chi context
		req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus/GPU-001/telemetry", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "GPU-001")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler.GetGPUTelemetry(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.TelemetryListResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Len(t, resp.Telemetry, 2)
		assert.Equal(t, 2, resp.Count)
		assert.Equal(t, 100, resp.TotalCount)
		assert.Equal(t, "GPU-001", resp.GPUUUID)
	})

	t.Run("GPU not found", func(t *testing.T) {
		repo := &mockRepository{gpus: []models.GPU{}}
		handler := newTestHandler(repo)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus/GPU-999/telemetry", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "GPU-999")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler.GetGPUTelemetry(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var resp models.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp.Error, "not found")
	})

	t.Run("with time filters", func(t *testing.T) {
		repo := &mockRepository{
			gpus:       []models.GPU{testGPU},
			telemetry:  testTelemetry,
			totalCount: 2,
		}
		handler := newTestHandler(repo)

		// Use UTC format to avoid URL encoding issues with timezone offset
		startTime := now.Add(-time.Hour).UTC().Format(time.RFC3339)
		endTime := now.UTC().Format(time.RFC3339)

		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/gpus/GPU-001/telemetry?start_time="+startTime+"&end_time="+endTime, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "GPU-001")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler.GetGPUTelemetry(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.TelemetryListResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.NotNil(t, resp.StartTime)
		assert.NotNil(t, resp.EndTime)
	})

	t.Run("with pagination", func(t *testing.T) {
		repo := &mockRepository{
			gpus:       []models.GPU{testGPU},
			telemetry:  testTelemetry,
			totalCount: 1000,
		}
		handler := newTestHandler(repo)

		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/gpus/GPU-001/telemetry?limit=50&offset=100", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "GPU-001")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()
		handler.GetGPUTelemetry(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", "2026-01-28T15:30:00Z", false},
		{"RFC3339 with offset", "2026-01-28T15:30:00+05:30", false},
		{"RFC3339Nano", "2026-01-28T15:30:00.123456789Z", false},
		{"DateTime", "2026-01-28T15:30:00", false},
		{"Date only", "2026-01-28", false},
		{"Unix timestamp", "1706454600", false},
		{"Invalid", "not-a-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseTime(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
