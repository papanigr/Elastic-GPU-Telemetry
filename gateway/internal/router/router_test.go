package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gpu-telemetry-pipeline/gateway/internal/handlers"
	"github.com/gpu-telemetry-pipeline/gateway/internal/models"
	"github.com/rs/zerolog"
)

// mockRepository for testing
type mockRepository struct {
	gpus       []models.GPU
	telemetry  []models.GPUTelemetry
	totalCount int
}

func (m *mockRepository) GetGPUs(ctx context.Context) ([]models.GPU, error) {
	return m.gpus, nil
}

func (m *mockRepository) GetGPUByUUID(ctx context.Context, uuid string) (*models.GPU, error) {
	for _, gpu := range m.gpus {
		if gpu.UUID == uuid {
			return &gpu, nil
		}
	}
	return nil, nil
}

func (m *mockRepository) GetTelemetry(ctx context.Context, filter models.TelemetryFilter) ([]models.GPUTelemetry, error) {
	return m.telemetry, nil
}

func (m *mockRepository) CountTelemetry(ctx context.Context, filter models.TelemetryFilter) (int, error) {
	return m.totalCount, nil
}

func (m *mockRepository) Ping(ctx context.Context) error {
	return nil
}

func (m *mockRepository) Close() error {
	return nil
}

func TestNew(t *testing.T) {
	logger := zerolog.Nop()
	repo := &mockRepository{}
	handler := handlers.NewHandler(repo, logger, 100, 1000)

	r := New(handler, logger)
	if r == nil {
		t.Fatal("Expected router to be created")
	}
}

func TestRouter_HealthEndpoint(t *testing.T) {
	logger := zerolog.Nop()
	repo := &mockRepository{}
	handler := handlers.NewHandler(repo, logger, 100, 1000)
	r := New(handler, logger)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRouter_GPUsEndpoint(t *testing.T) {
	logger := zerolog.Nop()
	now := time.Now()
	repo := &mockRepository{
		gpus: []models.GPU{
			{UUID: "GPU-001", GPUIndex: 0, Hostname: "host1", LastSeen: now},
		},
	}
	handler := handlers.NewHandler(repo, logger, 100, 1000)
	r := New(handler, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRouter_TelemetryEndpoint(t *testing.T) {
	logger := zerolog.Nop()
	now := time.Now()
	repo := &mockRepository{
		gpus: []models.GPU{
			{UUID: "GPU-001", GPUIndex: 0, Hostname: "host1", LastSeen: now},
		},
		telemetry: []models.GPUTelemetry{
			{ID: "1", UUID: "GPU-001", Timestamp: now, Value: 50.0},
		},
		totalCount: 1,
	}
	handler := handlers.NewHandler(repo, logger, 100, 1000)
	r := New(handler, logger)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus/GPU-001/telemetry", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRouter_SwaggerEndpoint(t *testing.T) {
	logger := zerolog.Nop()
	repo := &mockRepository{}
	handler := handlers.NewHandler(repo, logger, 100, 1000)
	r := New(handler, logger)

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Swagger should be available (might redirect or return 200)
	if w.Code != http.StatusOK && w.Code != http.StatusMovedPermanently && w.Code != http.StatusFound {
		t.Errorf("Expected swagger endpoint to be available, got status %d", w.Code)
	}
}

func TestRouter_CORS(t *testing.T) {
	logger := zerolog.Nop()
	repo := &mockRepository{}
	handler := handlers.NewHandler(repo, logger, 100, 1000)
	r := New(handler, logger)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/gpus", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected CORS header to be set")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS, got %d", w.Code)
	}
}

func TestRouter_NotFound(t *testing.T) {
	logger := zerolog.Nop()
	repo := &mockRepository{}
	handler := handlers.NewHandler(repo, logger, 100, 1000)
	r := New(handler, logger)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}
