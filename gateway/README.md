# API Gateway

REST API service for querying GPU telemetry data. Part of the GPU Telemetry Pipeline.

## Overview

The API Gateway provides REST endpoints for:
- Listing all unique GPUs in the system
- Querying telemetry data for specific GPUs
- Filtering telemetry by time range
- Pagination support for large datasets

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/api/v1/gpus` | List all GPUs |
| GET | `/api/v1/gpus/{id}/telemetry` | Get all telemetry for a GPU |
| GET | `/api/v1/gpus/{id}/telemetry?start_time=...&end_time=...` | Get telemetry with time filter |
| GET | `/swagger/index.html` | Swagger UI (interactive docs) |
| GET | `/swagger/doc.json` | OpenAPI spec (JSON) |

### Query Parameters for Telemetry

| Parameter | Type | Description |
|-----------|------|-------------|
| `start_time` | string | Start time filter (inclusive) |
| `end_time` | string | End time filter (inclusive) |
| `limit` | int | Max records (default 100, max 1000) |
| `offset` | int | Skip records (for pagination) |

### Supported Time Formats

```
Simple:      2026-01-28T15:30:00     ← Recommended (no timezone needed)
Date only:   2026-01-28              ← For full day queries
RFC3339:     2026-01-28T15:30:00Z    ← With UTC timezone
With offset: 2026-01-28T15:30:00+05:30
Unix:        1706454600              ← Unix timestamp (seconds)
```

## Quick Start

### Prerequisites

- PostgreSQL running with telemetry data (see `../db/`)
- Go 1.21+

### Run Locally

```bash
# Start PostgreSQL first
cd ../db && make up && make wait

# Run the gateway
cd ../gateway
make run
```

### Test Endpoints

```bash
# Health check
curl http://localhost:8080/health

# List GPUs
curl http://localhost:8080/api/v1/gpus

# Get telemetry for a GPU
curl http://localhost:8080/api/v1/gpus/GPU-5fd4f087-86f3-7a43-b711-4771313afc50/telemetry

# With time filter (simple format - no Z needed)
curl "http://localhost:8080/api/v1/gpus/GPU-5fd4f087-86f3-7a43-b711-4771313afc50/telemetry?start_time=2026-01-28T00:00:00&end_time=2026-01-28T23:59:59"

# Or just use date (gets full day)
curl "http://localhost:8080/api/v1/gpus/GPU-5fd4f087-86f3-7a43-b711-4771313afc50/telemetry?start_time=2026-01-28&end_time=2026-01-28"

# With pagination
curl "http://localhost:8080/api/v1/gpus/GPU-5fd4f087-86f3-7a43-b711-4771313afc50/telemetry?limit=50&offset=0"

# OpenAPI spec
curl http://localhost:8080/openapi.yaml
```

## Configuration

All configuration via environment variables with `GATEWAY_` prefix:

| Variable | Default | Description |
|----------|---------|-------------|
| `GATEWAY_HTTP_PORT` | 8080 | HTTP server port |
| `GATEWAY_DATABASE_URL` | postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable | PostgreSQL connection string |
| `GATEWAY_DB_MAX_OPEN_CONNS` | 25 | Max open database connections |
| `GATEWAY_DB_MAX_IDLE_CONNS` | 5 | Max idle connections |
| `GATEWAY_DB_CONN_MAX_LIFE` | 5m | Connection max lifetime |
| `GATEWAY_DEFAULT_PAGE_SIZE` | 100 | Default pagination limit |
| `GATEWAY_MAX_PAGE_SIZE` | 1000 | Maximum pagination limit |
| `GATEWAY_LOG_LEVEL` | info | Log level (debug, info, warn, error) |
| `GATEWAY_READ_TIMEOUT` | 30s | HTTP read timeout |
| `GATEWAY_WRITE_TIMEOUT` | 30s | HTTP write timeout |
| `GATEWAY_SHUTDOWN_TIMEOUT` | 10s | Graceful shutdown timeout |

## Auto-Generated OpenAPI Spec

The OpenAPI specification is **auto-generated** from code annotations using [swaggo/swag](https://github.com/swaggo/swag).

**How it works:**
1. Annotations are added to handlers (e.g., `// @Summary`, `// @Param`)
2. Running `make swagger` generates `docs/swagger.json` and `docs/swagger.yaml`
3. Swagger UI is served at `/swagger/index.html`

**Regenerate after code changes:**
```bash
make swagger
```

**Access documentation:**
- Swagger UI: http://localhost:8080/swagger/index.html
- JSON spec: http://localhost:8080/swagger/doc.json

## Project Structure

```
gateway/
├── cmd/
│   └── main.go              # Entry point + API annotations
├── docs/                    # Auto-generated Swagger docs
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── handlers/
│   │   ├── handlers.go      # HTTP handlers + annotations
│   │   └── handlers_test.go # Handler tests
│   ├── models/
│   │   └── models.go        # Data models
│   ├── repository/
│   │   ├── repository.go    # Database access layer
│   │   └── repository_test.go
│   └── router/
│       └── router.go        # HTTP routing + OpenAPI spec
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make swagger` | Generate OpenAPI spec (auto-generated) |
| `make build` | Generate swagger + build binary |
| `make run` | Run with PostgreSQL connection |
| `make test` | Run unit tests |
| `make test-coverage` | Run tests with coverage report |
| `make docker-build` | Build Docker image |
| `make docker-run` | Run Docker container |
| `make lint` | Run linter |
| `make fmt` | Format code |
| `make verify` | Run all checks |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    API Gateway                          │
├─────────────────────────────────────────────────────────┤
│  ┌─────────┐   ┌──────────┐   ┌────────────┐           │
│  │ Router  │──▶│ Handlers │──▶│ Repository │           │
│  │ (chi)   │   │          │   │ (interface)│           │
│  └─────────┘   └──────────┘   └────────────┘           │
│       │                              │                  │
│       ▼                              ▼                  │
│  ┌─────────┐                  ┌────────────┐           │
│  │OpenAPI  │                  │ PostgreSQL │           │
│  │  Spec   │                  │   (sqlx)   │           │
│  └─────────┘                  └────────────┘           │
└─────────────────────────────────────────────────────────┘
```

### Testability

The codebase is designed for easy unit testing:

1. **Repository Interface**: `repository.Repository` interface allows mocking
2. **Dependency Injection**: Handlers receive dependencies via constructor
3. **Mock Repository**: `repository_test.go` includes `MockRepository`
4. **HTTP Testing**: `httptest` used for handler testing

Example test:
```go
func TestGetGPUs(t *testing.T) {
    mock := &mockRepository{
        gpus: []models.GPU{{UUID: "GPU-001", ...}},
    }
    handler := handlers.NewHandler(mock, logger, 100, 1000)
    
    req := httptest.NewRequest(http.MethodGet, "/api/v1/gpus", nil)
    w := httptest.NewRecorder()
    
    handler.GetGPUs(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
}
```

## Response Examples

### GET /api/v1/gpus

```json
{
  "gpus": [
    {
      "uuid": "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
      "gpu_index": 0,
      "device": "nvidia0",
      "model_name": "NVIDIA H100 80GB HBM3",
      "hostname": "mtv5-dgx1-hgpu-031",
      "last_seen": "2026-01-28T15:30:00Z"
    }
  ],
  "count": 1
}
```

### GET /api/v1/gpus/{id}/telemetry

```json
{
  "telemetry": [
    {
      "id": "abc123",
      "timestamp": "2026-01-28T15:30:00Z",
      "metric_name": "DCGM_FI_DEV_GPU_UTIL",
      "gpu_index": 0,
      "device": "nvidia0",
      "uuid": "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
      "model_name": "NVIDIA H100 80GB HBM3",
      "hostname": "mtv5-dgx1-hgpu-031",
      "value": 75.5,
      "received_at": "2026-01-28T15:30:01Z"
    }
  ],
  "count": 1,
  "total_count": 100,
  "gpu_uuid": "GPU-5fd4f087-86f3-7a43-b711-4771313afc50",
  "start_time": "2026-01-28T00:00:00Z",
  "end_time": "2026-01-28T23:59:59Z"
}
```

### GET /health

```json
{
  "status": "healthy",
  "database": "healthy",
  "version": "1.0.0"
}
```

## Independence

This service is **completely independent**:

| Aspect | Description |
|--------|-------------|
| **No shared code** | No imports from `collector/`, `mq/`, or `streamer/` |
| **Own go.mod** | Independent dependency management |
| **Own Dockerfile** | Builds and deploys independently |
| **Runtime only** | Connects to PostgreSQL via connection string |

## Kubernetes Deployment

```yaml
env:
  - name: GATEWAY_DATABASE_URL
    value: "postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres-headless:5432/telemetry?sslmode=disable"
  - name: GATEWAY_HTTP_PORT
    value: "8080"
```
