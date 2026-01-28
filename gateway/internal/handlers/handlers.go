package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gpu-telemetry-pipeline/gateway/internal/models"
	"github.com/gpu-telemetry-pipeline/gateway/internal/repository"
	"github.com/rs/zerolog"
)

const (
	// Version is the API version.
	Version = "1.0.0"
)

// Handler holds the HTTP handlers and their dependencies.
type Handler struct {
	repo            repository.Repository
	logger          zerolog.Logger
	defaultPageSize int
	maxPageSize     int
}

// NewHandler creates a new Handler with the given dependencies.
func NewHandler(repo repository.Repository, logger zerolog.Logger, defaultPageSize, maxPageSize int) *Handler {
	return &Handler{
		repo:            repo,
		logger:          logger.With().Str("component", "handler").Logger(),
		defaultPageSize: defaultPageSize,
		maxPageSize:     maxPageSize,
	}
}

// Health godoc
// @Summary      Health check
// @Description  Returns the health status of the API and its dependencies
// @Tags         Health
// @Produce      json
// @Success      200  {object}  models.HealthResponse
// @Router       /health [get]
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dbStatus := "healthy"
	if err := h.repo.Ping(ctx); err != nil {
		dbStatus = "unhealthy"
		h.logger.Warn().Err(err).Msg("Database health check failed")
	}

	status := "healthy"
	if dbStatus != "healthy" {
		status = "degraded"
	}

	h.respondJSON(w, http.StatusOK, models.HealthResponse{
		Status:   status,
		Database: dbStatus,
		Version:  Version,
	})
}

// GetGPUs godoc
// @Summary      List all GPUs
// @Description  Returns a list of all unique GPUs that have reported telemetry
// @Tags         GPUs
// @Produce      json
// @Success      200  {object}  models.GPUListResponse
// @Failure      500  {object}  models.ErrorResponse
// @Router       /api/v1/gpus [get]
func (h *Handler) GetGPUs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	gpus, err := h.repo.GetGPUs(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to get GPUs")
		h.respondError(w, http.StatusInternalServerError, "Failed to retrieve GPUs", err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, models.GPUListResponse{
		GPUs:  gpus,
		Count: len(gpus),
	})
}

// GetGPUTelemetry godoc
// @Summary      Get GPU telemetry
// @Description  Returns telemetry data for a specific GPU, ordered by timestamp descending
// @Tags         Telemetry
// @Produce      json
// @Param        id          path      string  true   "GPU UUID (e.g., GPU-5fd4f087-86f3-7a43-b711-4771313afc50)"
// @Param        start_time  query     string  false  "Start time filter (formats: 2026-01-28T00:00:00, 2026-01-28, or Unix timestamp)"
// @Param        end_time    query     string  false  "End time filter (formats: 2026-01-28T23:59:59, 2026-01-28, or Unix timestamp)"
// @Param        limit       query     int     false  "Max records to return (default 100, max 1000)"
// @Param        offset      query     int     false  "Number of records to skip (for pagination)"
// @Success      200  {object}  models.TelemetryListResponse
// @Failure      400  {object}  models.ErrorResponse
// @Failure      404  {object}  models.ErrorResponse
// @Failure      500  {object}  models.ErrorResponse
// @Router       /api/v1/gpus/{id}/telemetry [get]
func (h *Handler) GetGPUTelemetry(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get GPU UUID from URL
	gpuUUID := chi.URLParam(r, "id")
	if gpuUUID == "" {
		h.respondError(w, http.StatusBadRequest, "GPU ID is required", "")
		return
	}

	// Check if GPU exists
	gpu, err := h.repo.GetGPUByUUID(ctx, gpuUUID)
	if err != nil {
		h.logger.Error().Err(err).Str("gpu_uuid", gpuUUID).Msg("Failed to get GPU")
		h.respondError(w, http.StatusInternalServerError, "Failed to retrieve GPU", err.Error())
		return
	}
	if gpu == nil {
		h.respondError(w, http.StatusNotFound, "GPU not found", gpuUUID)
		return
	}

	// Parse query parameters
	filter, err := h.parseTelemetryFilter(r, gpuUUID)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid query parameters", err.Error())
		return
	}

	// Get telemetry
	telemetry, err := h.repo.GetTelemetry(ctx, filter)
	if err != nil {
		h.logger.Error().Err(err).Str("gpu_uuid", gpuUUID).Msg("Failed to get telemetry")
		h.respondError(w, http.StatusInternalServerError, "Failed to retrieve telemetry", err.Error())
		return
	}

	// Get total count (for pagination)
	totalCount, err := h.repo.CountTelemetry(ctx, filter)
	if err != nil {
		h.logger.Warn().Err(err).Msg("Failed to count telemetry")
		totalCount = len(telemetry)
	}

	h.respondJSON(w, http.StatusOK, models.TelemetryListResponse{
		Telemetry:  telemetry,
		Count:      len(telemetry),
		TotalCount: totalCount,
		GPUUUID:    gpuUUID,
		StartTime:  filter.StartTime,
		EndTime:    filter.EndTime,
	})
}

// parseTelemetryFilter extracts filter parameters from the request.
func (h *Handler) parseTelemetryFilter(r *http.Request, gpuUUID string) (models.TelemetryFilter, error) {
	filter := models.TelemetryFilter{
		GPUUUID: gpuUUID,
		Limit:   h.defaultPageSize,
		Offset:  0,
	}

	// Parse start_time
	if startStr := r.URL.Query().Get("start_time"); startStr != "" {
		start, err := parseTime(startStr)
		if err != nil {
			return filter, err
		}
		filter.StartTime = &start
	}

	// Parse end_time
	if endStr := r.URL.Query().Get("end_time"); endStr != "" {
		end, err := parseTime(endStr)
		if err != nil {
			return filter, err
		}
		filter.EndTime = &end
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			return filter, err
		}
		if limit > h.maxPageSize {
			limit = h.maxPageSize
		}
		filter.Limit = limit
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return filter, err
		}
		filter.Offset = offset
	}

	return filter, nil
}

// parseTime parses a time string in various formats.
func parseTime(s string) (time.Time, error) {
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	// Try RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}

	// Try simple date-time format
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t, nil
	}

	// Try date only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}

	// Try Unix timestamp
	if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(ts, 0), nil
	}

	return time.Time{}, &time.ParseError{Value: s}
}

// respondJSON writes a JSON response.
func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error().Err(err).Msg("Failed to encode JSON response")
	}
}

// respondError writes an error response.
func (h *Handler) respondError(w http.ResponseWriter, status int, message string, details string) {
	h.respondJSON(w, status, models.ErrorResponse{
		Error:   message,
		Code:    status,
		Details: details,
	})
}
