# Telemetry Collector

The Telemetry Collector consumes GPU telemetry data from the custom Message Queue and persists it to PostgreSQL. It supports dynamic scaling with consumer groups for load balancing.

## Overview

- **Consumes** telemetry via gRPC from MQ broker
- **Parses** JSON telemetry payloads
- **Persists** to PostgreSQL (or prints to console in dry-run mode)
- **Acknowledges** processed messages
- **Scales** horizontally with consumer groups

## Architecture

```
┌─────────────┐     gRPC      ┌─────────────┐      SQL       ┌────────────┐
│  MQ Broker  │──────────────▶│  Collector  │───────────────▶│ PostgreSQL │
│   :8081     │   (consume)   │             │   (persist)    │            │
└─────────────┘               └─────────────┘                └────────────┘
```

## gRPC Communication

The collector uses the following gRPC RPCs:

| RPC | Purpose |
|-----|---------|
| `Subscribe` | Register consumer with consumer group |
| `Consume` | Fetch batch of messages |
| `Ack` | Acknowledge processed messages |
| `Health` | Check MQ broker health |

### Consumer Groups

Multiple collector instances can join the same consumer group:

```
Collector-1 ─┐
             ├──▶ Consumer Group "telemetry-collectors" ──▶ Topic "gpu-telemetry"
Collector-2 ─┘
```

Messages are distributed among consumers in the group (at-least-once delivery).

## Worker Pool Architecture

Each collector instance uses a worker pool for concurrent processing:

```
                          ┌─────────────┐
                     ┌───▶│  Worker 1   │───┐
┌─────────┐          │    └─────────────┘   │    ┌──────────┐
│ Fetcher │──batch──▶│    ┌─────────────┐   ├───▶│ Acker    │
│  Loop   │          ├───▶│  Worker 2   │───┤    │ (sends   │
└─────────┘          │    └─────────────┘   │    │  ACKs)   │
                     │    ┌─────────────┐   │    └──────────┘
                     └───▶│  Worker 3   │───┘
                          └─────────────┘
                                │
                                ▼
                          ┌─────────────┐
                          │ PostgreSQL  │
                          └─────────────┘
```

| Component | Description |
|-----------|-------------|
| **Fetcher** | Polls MQ for message batches |
| **Workers** | Parse JSON, persist to DB (concurrent) |
| **Acker** | Sends ACKs back to MQ after processing |

Configure with `COLLECTOR_NUM_WORKERS` (default: 3).

## Project Structure

```
collector/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── config.go            # Configuration management
│   ├── consumer/
│   │   └── consumer.go      # MQ consumer + fetch/ack logic
│   ├── models/
│   │   └── telemetry.go     # Data models
│   ├── repository/
│   │   └── repository.go    # Database interface + implementations
│   └── worker/
│       └── pool.go          # Concurrent worker pool
├── proto/
│   └── mq.proto             # gRPC service definition (independent copy)
├── pkg/
│   └── pb/                  # Generated protobuf code
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

## Configuration

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `COLLECTOR_MQ_BROKER_ADDR` | MQ broker gRPC address | `mq-broker:8081` |
| `COLLECTOR_TOPIC` | Topic to consume from | `gpu-telemetry` |
| `COLLECTOR_CONSUMER_GROUP` | Consumer group name | `telemetry-collectors` |
| `COLLECTOR_CONSUMER_ID` | Unique consumer ID | Auto-generated UUID |
| `COLLECTOR_BATCH_SIZE` | Messages per batch | `20` |
| `COLLECTOR_POLL_INTERVAL` | Polling interval | `500ms` |
| `COLLECTOR_NUM_WORKERS` | Worker pool size | `3` |
| `COLLECTOR_DB_ENABLED` | Enable PostgreSQL persistence | `false` |
| `COLLECTOR_DATABASE_URL` | PostgreSQL connection string | See below |
| `COLLECTOR_LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |

Default database URL: `postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable`

## Prerequisites

If modifying `.proto` files:

```bash
# Install protoc
brew install protobuf

# Install Go plugins
make proto-install

# Generate code
make proto
```

## Build & Run

```bash
# Build
make build

# Run with MQ (requires MQ broker on localhost:8081)
make run

# Run in debug mode (more verbose, smaller batches)
make dry-run

# Run tests
make test
```

## Quick Start

```bash
# Terminal 1: Start MQ broker
cd ../mq && make run

# Terminal 2: Start streamer
cd ../streamer && make run

# Terminal 3: Start collector
cd ../collector && make run
```

## Example Output (dry-run mode)

```
INF Starting Telemetry Collector mq_broker_addr=localhost:8081 topic=gpu-telemetry
INF Connecting to MQ broker addr=localhost:8081
INF MQ broker is healthy status=healthy
INF Subscribed to topic topic=gpu-telemetry group=telemetry-collectors
INF Starting consumer batch_size=10 poll_interval=1s

INF === Records to be saved to PostgreSQL === count=10
INF GPU Telemetry Record gpu_uuid=GPU-abc-123 temperature=65.5 power_usage=180.2 utilization=75.3
INF GPU Telemetry Record gpu_uuid=GPU-def-456 temperature=72.1 power_usage=220.5 utilization=92.1
...
INF === End of batch === count=10

INF Batch processed successfully consumed=10 parsed=10 acked=10
INF Consumer statistics messages_consumed=100 batches_processed=10 messages_per_second=10.0
```

## Docker

```bash
# Build image
make docker

# Run container (connects to host network)
make docker-run

# Or with custom MQ address
docker run --rm -it \
  -e COLLECTOR_MQ_BROKER_ADDR=mq-broker:8081 \
  -e COLLECTOR_DB_ENABLED=false \
  telemetry-collector:latest
```

## Scaling

The collector supports horizontal scaling via consumer groups:

```bash
# Run multiple instances (each gets unique consumer ID)
COLLECTOR_CONSUMER_ID=collector-1 make run &
COLLECTOR_CONSUMER_ID=collector-2 make run &
COLLECTOR_CONSUMER_ID=collector-3 make run &
```

In Kubernetes, deploy multiple replicas - each pod automatically joins the consumer group.

## Makefile Targets

| Target | Description |
|--------|-------------|
| `build` | Build the binary |
| `clean` | Remove binary and generated files |
| `test` | Run tests |
| `test-cover` | Run tests with coverage |
| `proto` | Generate protobuf code |
| `proto-install` | Install protoc plugins |
| `run` | Run with MQ + PostgreSQL (production) |
| `dry-run` | Run without DB (console output only) |
| `run-high` | Run with high throughput + PostgreSQL |
| `docker` | Build Docker image |
| `docker-run` | Run Docker container |
| `lint` | Run linter |

## Independence

The collector is **completely independent** from other services:

| Aspect | Description |
|--------|-------------|
| **Own go.mod** | Independent Go module |
| **Own Dockerfile** | Builds without other services |
| **Own proto files** | No imports from `mq/` |
| **No db/ imports** | Connects to PostgreSQL via connection string |

### Runtime Dependencies

| Service | Connection | Required At |
|---------|------------|-------------|
| MQ Broker | gRPC (`COLLECTOR_MQ_BROKER_ADDR`) | Runtime |
| PostgreSQL | Connection string (`COLLECTOR_DATABASE_URL`) | Runtime (optional) |

In Kubernetes, services discover each other via DNS (e.g., `mq-broker:8081`, `postgres-headless:5432`).

## Database Schema

The collector expects the `gpu_telemetry` table (created by `db/` migrations):

| Column | Type | Description |
|--------|------|-------------|
| id | VARCHAR(36) | Primary key |
| timestamp | TIMESTAMPTZ | When metric was collected |
| metric_name | VARCHAR(64) | DCGM metric name |
| gpu_index | INTEGER | GPU index (0-7) |
| device | VARCHAR(32) | Linux device |
| uuid | VARCHAR(64) | Unique GPU ID |
| model_name | VARCHAR(128) | GPU model |
| hostname | VARCHAR(128) | Host server |
| container | VARCHAR(128) | Container name |
| pod | VARCHAR(128) | K8s pod name |
| namespace | VARCHAR(64) | K8s namespace |
| value | DOUBLE PRECISION | Metric value |
| labels_raw | TEXT | Raw Prometheus labels |
| received_at | TIMESTAMPTZ | When collector received |
| message_id | VARCHAR(36) | MQ message ID |

The schema is defined in `db/migrations/` but there's **no code dependency** - the collector just needs the table to exist at runtime.
