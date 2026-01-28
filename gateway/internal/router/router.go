package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gpu-telemetry-pipeline/gateway/internal/handlers"
	"github.com/rs/zerolog"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// New creates a new chi router with all routes configured.
func New(h *handlers.Handler, logger zerolog.Logger) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(zerologMiddleware(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS headers
	r.Use(corsMiddleware)

	// Health check (outside versioned API)
	r.Get("/health", h.Health)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// GPU endpoints
		r.Get("/gpus", h.GetGPUs)
		r.Get("/gpus/{id}/telemetry", h.GetGPUTelemetry)
	})

	// Swagger UI - auto-generated documentation
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"), // The url pointing to API definition
	))

	return r
}

// zerologMiddleware logs requests using zerolog.
func zerologMiddleware(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Dur("duration", time.Since(start)).
				Str("remote_addr", r.RemoteAddr).
				Str("request_id", middleware.GetReqID(r.Context())).
				Msg("Request completed")
		})
	}
}

// corsMiddleware adds CORS headers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
