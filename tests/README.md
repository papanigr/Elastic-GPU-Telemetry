# Integration Tests

System/integration tests for the GPU Telemetry Pipeline. These tests verify the end-to-end behavior of all components working together.

## Overview

The integration tests are **black-box tests** that interact with services via HTTP/gRPC APIs only. They don't import any code from the main services.

```
┌─────────────────────────────────────────────────────────────────┐
│                    Integration Test Suite                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────┐    ┌─────┐    ┌───────────┐    ┌──────────────┐  │
│  │ Streamer │───▶│ MQ  │───▶│ Collector │───▶│  PostgreSQL  │  │
│  └──────────┘    └─────┘    └───────────┘    └──────────────┘  │
│                                                      │          │
│                                              ┌───────▼───────┐  │
│                                              │    Gateway    │  │
│                                              └───────────────┘  │
│                                                      │          │
│                                              ┌───────▼───────┐  │
│                                              │  Test Client  │  │
│                                              │  (Assertions) │  │
│                                              └───────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Prerequisites

- **Container runtime**: Podman (recommended) or Docker - auto-detected, no compose needed
- **Go 1.21+**
- **curl and jq** (for health checks)

## Quick Start

```bash
# 1. Start the test environment
make up

# 2. Run all integration tests
make test

# 3. Stop the test environment
make down
```

## Test Environment

The test environment runs all services with test-specific ports (different from dev to avoid conflicts):

| Service    | Port  | Description |
|------------|-------|-------------|
| PostgreSQL | 5433  | PostgreSQL for test data |
| MQ HTTP    | 8083  | Message Queue HTTP API |
| MQ gRPC    | 8084  | Message Queue gRPC API |
| Gateway    | 8085  | REST API Gateway |

## Test Categories

### End-to-End Tests (`e2e_flow_test.go`)

Verify complete data flow through all components:
- Publish telemetry → MQ → Collector → PostgreSQL → Gateway API

```bash
make test-e2e
```

### API Tests (`api_test.go`)

Verify Gateway API endpoints:
- Health checks
- List GPUs
- Query telemetry
- Time filtering
- Pagination
- Error handling

```bash
make test-api
```

### MQ Tests (`mq_test.go`)

Verify Message Queue behavior:
- Publishing messages
- Topic statistics
- Graceful degradation
- High throughput

```bash
make test-mq
```

### Scaling Tests (`scaling_test.go`)

Verify system behavior under load:
- Concurrent publishers
- High volume ingestion
- Multiple consumer groups
- System resilience

```bash
make test-scaling
```

## Makefile Targets

Run `make help` to see all available targets. Uses podman or docker automatically.

| Command | Description | Requirements |
|---------|-------------|--------------|
| `make help` | Show all available targets | - |
| `make check-engine` | Verify container engine is available | - |
| **Setup** | | |
| `make deps` | Install test dependencies (`go mod tidy`) | Go installed |
| `make network` | Create test network | Container runtime |
| `make build` | Build all service images (MQ, Collector, Gateway, Streamer) | Container runtime |
| **Environment** | | |
| `make up` | Start all test services | Container runtime |
| `make down` | Stop all test services | Container runtime |
| `make up-postgres` | Start only PostgreSQL container | Container runtime |
| `make up-mq` | Start only MQ container | Container runtime |
| `make up-collector` | Start only Collector container | Container runtime |
| `make up-gateway` | Start only Gateway container | Container runtime |
| **Monitoring** | | |
| `make status` | Show status of test services | Container runtime |
| `make health` | Check health of all services | Container runtime, curl, jq |
| `make logs` | Show logs from all services (last 20 lines) | Container runtime |
| `make logs-mq` | Follow MQ logs | Container runtime |
| `make logs-collector` | Follow Collector logs | Container runtime |
| `make logs-gateway` | Follow Gateway logs | Container runtime |
| **Testing** | | |
| `make test` | Run all integration tests | Environment running |
| `make test-short` | Run tests in short mode (skip slow tests) | Environment running |
| `make test-e2e` | Run only end-to-end tests | Environment running |
| `make test-api` | Run only API tests | Environment running |
| `make test-mq` | Run only MQ tests | Environment running |
| `make test-scaling` | Run only scaling tests | Environment running |
| `make test-all` | Start env, run all tests, stop env | Container runtime |
| **Cleanup** | | |
| `make clean` | Clean up test artifacts, containers, and network | Container runtime |

## Configuration

Tests can be configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_GATEWAY_URL` | `http://localhost:8085` | Gateway URL |
| `TEST_MQ_URL` | `http://localhost:8083` | MQ HTTP URL |
| `TEST_MQ_GRPC_ADDR` | `localhost:8084` | MQ gRPC address |
| `TEST_POSTGRES_URL` | `postgres://postgres:postgres@localhost:5433/telemetry_test` | PostgreSQL URL |

## Test Data

Test data is located in `testdata/test_metrics.csv`:
- 3 unique GPUs
- 5 metrics per GPU
- Different hostnames

## Running Specific Tests

```bash
# Run a specific test
go test -tags=integration -v -run "TestEndToEndTelemetryFlow" ./integration/...

# Run tests matching a pattern
go test -tags=integration -v -run ".*GPU.*" ./integration/...

# Run with verbose output and longer timeout
go test -tags=integration -v -timeout 10m ./integration/...
```

## Troubleshooting

### Services not starting

```bash
# Check container status
make status

# Check logs
make logs

# Rebuild containers
make build
```

### Tests failing with connection errors

```bash
# Check service health
make health

# Wait longer for services to start
sleep 30 && make test
```

### Database issues

```bash
# Reset the database
make down
make up
```

## Writing New Tests

1. Create a new test file in `integration/` with `//go:build integration` tag
2. Use the `TestClient` from `setup_test.go` for API calls
3. Use `RequireServicesHealthy` at the start of each test
4. Use `WaitForCondition` for async operations

Example:

```go
//go:build integration

package integration

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestMyFeature(t *testing.T) {
    cfg := DefaultTestConfig()
    client := NewTestClient(cfg)
    
    RequireServicesHealthy(t, client)
    
    // Your test logic here
    assert.True(t, true)
}
```

## CI/CD Integration

For CI/CD pipelines:

```yaml
integration-tests:
  script:
    - cd tests
    - make up
    - sleep 30
    - make test
    - make down
  after_script:
    - make down || true
```
