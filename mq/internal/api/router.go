package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gpu-telemetry-pipeline/mq/internal/broker"
	"github.com/rs/zerolog"
)

// NewRouter creates a new HTTP router with all routes configured.
func NewRouter(b *broker.Broker, logger zerolog.Logger) *chi.Mux {
	handler := NewHandler(b, logger)

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Health endpoint (returns JSON status)
	r.Get("/health", handler.Health)

	// API v1 routes (HTTP endpoints for message operations)
	// Streamer uses these HTTP endpoints, gRPC is also available on port 8081
	r.Route("/api/v1", func(r chi.Router) {
		// Topic operations
		r.Route("/topics/{topic}", func(r chi.Router) {
			// Publish
			r.Post("/messages", handler.Publish)

			// Subscribe
			r.Post("/subscribe", handler.Subscribe)

			// Consume (GET with query params)
			r.Get("/messages", handler.Consume)

			// Acknowledge
			r.Post("/ack", handler.Ack)
		})
	})

	// Admin routes
	r.Route("/admin", func(r chi.Router) {
		// List all topics
		r.Get("/topics", handler.ListTopics)

		r.Route("/topics/{topic}", func(r chi.Router) {
			// Topic stats
			r.Get("/stats", handler.GetTopicStats)

			// View messages
			r.Get("/messages", handler.GetMessages)

			// Delete all messages (purge)
			r.Delete("/messages", handler.PurgeMessages)

			// Delete specific message
			r.Delete("/messages/{messageID}", handler.DeleteMessage)

			// List consumers
			r.Get("/consumers", handler.GetConsumers)
		})
	})

	return r
}
