# MQ Broker Service

A custom in-memory message queue broker for the GPU Telemetry Pipeline.

## Overview

The MQ Broker provides:
- Topic-based pub/sub messaging
- Consumer groups with round-robin load balancing
- At-least-once delivery with acknowledgments
- **Dead Letter Queue (DLQ)** for failed message handling with auto-retry
- **Dual protocol support:** gRPC (port 8081) + HTTP (port 8082)
- Admin APIs for queue inspection and management
- Production-grade protobuf/gRPC implementation

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         MQ Broker                                │
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              gRPC Server (Port 8081)                        ││
│  │  Publish()  Subscribe()  Consume()  Ack()  Health()         ││
│  │  ← Fast binary protobuf for inter-service communication     ││
│  └─────────────────────────────────────────────────────────────┘│
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │              HTTP Server (Port 8082)                        ││
│  │  POST /api/v1/topics/{topic}/messages  → Publish            ││
│  │  GET  /api/v1/topics/{topic}/messages  → Consume            ││
│  │  POST /api/v1/topics/{topic}/ack       → Acknowledge        ││
│  │  GET  /admin/topics/{topic}/stats      → Admin Stats        ││
│  │  ← REST API for debugging and admin operations              ││
│  └─────────────────────────────────────────────────────────────┘│
│                              │                                   │
│  ┌───────────────────────────▼────────────────────────────────┐ │
│  │                    Broker Core                              │ │
│  │  Topics → Messages (in-memory) → Consumer Groups            │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘

Ports:
  - 8081: gRPC (streamer, collector use this)
  - 8082: HTTP (admin APIs, debugging)
```

## Prerequisites

To generate protobuf code locally:

```bash
# Install protoc (macOS)
brew install protobuf

# Install Go plugins (run once)
make proto-install
```

## Quick Start

```bash
# Generate proto files (first time or after proto changes)
make proto

# Build and run
make run
```

## Project Structure

```
mq/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── api/
│   │   ├── handlers.go      # HTTP handlers
│   │   └── router.go        # Route configuration
│   ├── broker/
│   │   ├── broker.go        # Core broker logic
│   │   ├── topic.go         # Topic management
│   │   └── consumer.go      # Consumer group management
│   └── config/
│       └── config.go        # Configuration
├── pkg/
│   └── models/
│       └── message.go       # Data models
├── Dockerfile
├── Makefile
├── go.mod
└── README.md
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MQ_GRPC_PORT` | `8081` | gRPC server port (inter-service) |
| `MQ_HTTP_PORT` | `8082` | HTTP server port (admin APIs) |
| `MQ_MAX_QUEUE_SIZE` | `10000` | Max messages per topic |
| `MQ_MAX_MESSAGE_AGE` | `5m` | Drop messages older than this |
| `MQ_ACK_TIMEOUT` | `30s` | Redeliver if no ACK within this time |
| `MQ_CLEANUP_INTERVAL` | `10s` | Cleanup routine interval |
| `MQ_LOG_LEVEL` | `info` | Log level |
| **Dead Letter Queue (DLQ)** | | |
| `MQ_DLQ_ENABLED` | `true` | Enable Dead Letter Queue |
| `MQ_MAX_RETRIES` | `3` | Max delivery attempts before moving to DLQ |
| `MQ_DLQ_MAX_RETRIES` | `3` | Max DLQ retry attempts before marking as dead |
| `MQ_DLQ_RETRY_DELAY` | `5m` | Delay between DLQ auto-retry attempts |

## REST API Reference

All HTTP endpoints are served on port **8082** by default.

### Core Endpoints

| Method | Endpoint | Description | Example |
|--------|----------|-------------|---------|
| GET | `/health` | Health check | `curl http://localhost:8082/health` |
| GET | `/swagger/index.html` | Swagger UI (interactive docs) | `open http://localhost:8082/swagger/index.html` |
| GET | `/swagger/doc.json` | OpenAPI spec (JSON) | `curl http://localhost:8082/swagger/doc.json` |

### Message Operations

| Method | Endpoint | Description | Example |
|--------|----------|-------------|---------|
| POST | `/api/v1/topics/{topic}/messages` | Publish a message | `curl -X POST http://localhost:8082/api/v1/topics/gpu-telemetry/messages -H "Content-Type: application/json" -d '{"payload": {"metric": "gpu_util", "value": 85}}'` |
| GET | `/api/v1/topics/{topic}/messages` | Consume messages | `curl "http://localhost:8082/api/v1/topics/gpu-telemetry/messages?consumer_id=c1&group=g1&max_messages=10"` |
| POST | `/api/v1/topics/{topic}/subscribe` | Subscribe consumer to topic | `curl -X POST http://localhost:8082/api/v1/topics/gpu-telemetry/subscribe -H "Content-Type: application/json" -d '{"consumer_id": "c1", "group": "g1"}'` |
| POST | `/api/v1/topics/{topic}/ack` | Acknowledge messages | `curl -X POST http://localhost:8082/api/v1/topics/gpu-telemetry/ack -H "Content-Type: application/json" -d '{"consumer_id": "c1", "message_ids": ["msg-id-1"]}'` |

### Admin Operations

| Method | Endpoint | Description | Example |
|--------|----------|-------------|---------|
| GET | `/admin/topics` | List all topics | `curl http://localhost:8082/admin/topics` |
| GET | `/admin/topics/{topic}/stats` | Get topic statistics | `curl http://localhost:8082/admin/topics/gpu-telemetry/stats` |
| GET | `/admin/topics/{topic}/messages` | View messages in topic | `curl "http://localhost:8082/admin/topics/gpu-telemetry/messages?limit=10"` |
| DELETE | `/admin/topics/{topic}/messages` | Purge all messages in topic | `curl -X DELETE http://localhost:8082/admin/topics/gpu-telemetry/messages` |
| DELETE | `/admin/topics/{topic}/messages/{id}` | Delete specific message | `curl -X DELETE http://localhost:8082/admin/topics/gpu-telemetry/messages/msg-id-1` |
| GET | `/admin/topics/{topic}/consumers` | List consumers for topic | `curl http://localhost:8082/admin/topics/gpu-telemetry/consumers` |

### Dead Letter Queue (DLQ) Admin Operations

Messages that fail delivery after `MQ_MAX_RETRIES` (default: 3) attempts are automatically moved to the DLQ.

| Method | Endpoint | Description | Example |
|--------|----------|-------------|---------|
| GET | `/admin/topics/{topic}/dlq/stats` | Get DLQ statistics | `curl http://localhost:8082/admin/topics/gpu-telemetry/dlq/stats` |
| GET | `/admin/topics/{topic}/dlq/messages` | View DLQ messages with failure details | `curl "http://localhost:8082/admin/topics/gpu-telemetry/dlq/messages?limit=10"` |
| POST | `/admin/topics/{topic}/dlq/replay` | Replay DLQ messages back to original topic | `curl -X POST http://localhost:8082/admin/topics/gpu-telemetry/dlq/replay` |
| DELETE | `/admin/topics/{topic}/dlq` | Purge all DLQ messages | `curl -X DELETE http://localhost:8082/admin/topics/gpu-telemetry/dlq` |

#### DLQ Message States

| State | Description |
|-------|-------------|
| `pending` | Message is waiting to be auto-retried (after `MQ_DLQ_RETRY_DELAY`) |
| `delivered` | Message was delivered to consumer, awaiting ACK |
| `dead` | Message exceeded `MQ_DLQ_MAX_RETRIES`, requires admin action |

#### DLQ Admin Workflow

```bash
# 1. Check if there are DLQ messages
curl http://localhost:8082/admin/topics/gpu-telemetry/dlq/stats

# 2. View failed messages to understand the issue
curl "http://localhost:8082/admin/topics/gpu-telemetry/dlq/messages?limit=10"

# 3. Fix the root cause (e.g., restart database, fix parsing bug)

# 4. Replay all pending DLQ messages back to main queue
curl -X POST http://localhost:8082/admin/topics/gpu-telemetry/dlq/replay

# 5. To replay specific messages only:
curl -X POST http://localhost:8082/admin/topics/gpu-telemetry/dlq/replay \
  -H "Content-Type: application/json" \
  -d '{"message_ids": ["msg-id-1", "msg-id-2"]}'

# 6. To replay dead messages (force):
curl -X POST http://localhost:8082/admin/topics/gpu-telemetry/dlq/replay \
  -H "Content-Type: application/json" \
  -d '{"force": true}'

# 7. Discard all DLQ messages (if data is not needed)
curl -X DELETE http://localhost:8082/admin/topics/gpu-telemetry/dlq
```

## Dead Letter Queue (DLQ) Architecture

The MQ Broker implements a hybrid DLQ approach with **automatic replay** for handling failed messages:

```
                          Main Queue                              DLQ
                              │                                    │
     Publish ───────────────► │                                    │
                              ▼                                    │
                        ┌──────────┐                               │
                        │ Pending  │                               │
                        └────┬─────┘                               │
                             │ Consume                             │
                             ▼                                     │
                        ┌──────────┐                               │
                        │Delivered │                               │
                        └────┬─────┘                               │
                   ┌─────────┴─────────┐                           │
              ACK  │                   │ No ACK (timeout)          │
                   ▼                   ▼                           │
              ┌────────┐         RetryCount++                      │
              │ Acked  │               │                           │
              │(remove)│    ┌──────────┴──────────┐                │
              └────────┘    │                     │                │
                     < MaxRetries         ≥ MaxRetries             │
                            │                     │                │
                            ▼                     ▼                │
                       ┌──────────┐    ┌────────────────────┐      │
                       │ Pending  │    │  Move to DLQ       │──────►
                       │(redeliver)│   └────────────────────┘      │
                       └──────────┘                                │
                            ▲                                      ▼
                            │                                ┌───────────┐
                            │                                │DLQ Pending│
                            │                                └─────┬─────┘
                            │                                      │
                            │          After DLQRetryDelay         │
                            │         (auto-replay to main)        │
                            │◄─────────────────────────────────────┘
                            │                                      │
                            │                           DLQRetryCount++
                            │                                      │
                            │                      ┌───────────────┴───────────────┐
                            │                      │                               │
                            │             < DLQMaxRetries                  ≥ DLQMaxRetries
                            │                      │                               │
                            │                      ▼                               ▼
                            │               ┌─────────────┐                 ┌──────────┐
                            └───────────────│Auto-Replay  │                 │  Dead    │
                                            │to Main Queue│                 │(admin)   │
                                            └─────────────┘                 └──────────┘
```

### Automatic Recovery Flow (No Admin Action Needed)

When the database goes down and comes back up:

```
1. Collector fails to save → No ACK sent
2. MQ redelivers after 30s (MQ_ACK_TIMEOUT)
3. After 3 failures → Message moves to DLQ
4. After 5 min → Message auto-replays back to main queue
5. Collector picks up message → DB is back → Saves successfully → ACK
6. Message removed ✓

If DB is still down after replay:
7. Message fails again → Back to DLQ
8. After 3 auto-replays → Message marked as Dead
9. Admin manually replays when ready
```

### Key Features

1. **Automatic retry**: Messages redelivered after `MQ_ACK_TIMEOUT` (30s)
2. **DLQ after max retries**: Messages move to DLQ after `MQ_MAX_RETRIES` (3) failures
3. **Auto-replay to main queue**: DLQ messages automatically replayed to original topic after `MQ_DLQ_RETRY_DELAY` (5m)
4. **Dead state**: After `MQ_DLQ_MAX_RETRIES` (3) auto-replays still failing, messages marked dead
5. **Admin replay**: Dead messages can be manually replayed with `force: true`

### Timeline Example (Default Settings)

| Time | Event |
|------|-------|
| 0:00 | Message published, DB goes down |
| 0:30 | First retry (no ACK after 30s) |
| 1:00 | Second retry |
| 1:30 | Third retry → Move to DLQ |
| 6:30 | First auto-replay to main queue |
| 7:00 | Fourth retry |
| 7:30 | Fifth retry → Back to DLQ |
| 12:30 | Second auto-replay |
| 18:30 | Third auto-replay |
| 19:00+ | If still failing → Marked as Dead |

### Why Auto-Replay Instead of Direct DLQ Consumer?

An alternative design would have the Collector subscribe directly to the DLQ topic. Here's why we chose auto-replay instead:

| Problem | Direct DLQ Consumer | Auto-Replay Design |
|---------|--------------------|--------------------|
| **Infinite loop** | If DB still down, message fails again → Where does it go? Another DLQ? | Message goes back to DLQ with `dlq_retry_count++`, stops after max retries |
| **Retry counting** | Complex to track retries across two subscriptions | Built-in `retry_count` and `dlq_retry_count` per message |
| **Dead state** | Need separate logic to stop infinite retries | Automatic `Dead` state after N replays |
| **Code complexity** | Collector handles 2 topics with different logic | Single consumer path, same code for all messages |
| **Ordering** | Mixed old DLQ + new messages | DLQ waits in holding area, then rejoins main flow |

**The auto-replay design treats DLQ as a temporary holding area, not a separate processing queue.**

```
DLQ Design: Holding Area with Auto-Replay
──────────────────────────────────────────
                    ┌─────────────┐
                    │    DLQ      │ ← No consumer needed
                    │  (holding)  │
                    └──────┬──────┘
                           │ After 5 min delay
                           ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Streamer   │────▶│ Main Queue  │────▶│  Collector  │
│             │     │             │     │ (single     │
└─────────────┘     └─────────────┘     │  consumer)  │
                                        └─────────────┘
```

## Makefile Targets

Run `make help` to see all available targets. Uses podman or docker automatically.

| Command | Description | Requirements |
|---------|-------------|--------------|
| `make help` | Show all available targets | - |
| **Build** | | |
| `make build` | Build the MQ broker binary (generates swagger first) | - |
| `make clean` | Remove binary, proto files, and coverage files | - |
| `make all` | Run proto, fmt, vet, test, and build | - |
| **Testing** | | |
| `make test` | Run all unit tests | - |
| `make test-cover` | Run tests with coverage (generates `coverage.html`) | - |
| **Code Quality** | | |
| `make fmt` | Format code with gofmt | - |
| `make vet` | Run go vet | - |
| `make mod-tidy` | Tidy Go modules (`go mod tidy`) | - |
| **Proto** | | |
| `make proto` | Generate Go code from `.proto` files | `protoc` installed |
| `make proto-install` | Install protoc Go plugins | Go installed |
| `make proto-check` | Check if proto files exist, generate if missing | - |
| **Swagger** | | |
| `make swagger` | Generate OpenAPI spec (auto-generated from annotations) | - |
| **Run** | | |
| `make run` | Run MQ broker locally (gRPC:8081, HTTP:8082) | - |
| **Docker** | | |
| `make docker` | Build Docker image (uses podman or docker) | Container runtime |
| `make docker-run` | Run container exposing ports 8081 and 8082 | Container runtime |

## Auto-Generated OpenAPI

The OpenAPI specification is **auto-generated** from code annotations using [swaggo/swag](https://github.com/swaggo/swag).

- Swagger UI: http://localhost:8082/swagger/index.html
- JSON spec: http://localhost:8082/swagger/doc.json

Regenerate after code changes:
```bash
make swagger
```

