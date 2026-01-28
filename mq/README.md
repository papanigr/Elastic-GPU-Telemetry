# MQ Broker Service

A custom in-memory message queue broker for the GPU Telemetry Pipeline.

## Overview

The MQ Broker provides:
- Topic-based pub/sub messaging
- Consumer groups with round-robin load balancing
- At-least-once delivery with acknowledgments
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

## API Endpoints

### Documentation (Auto-Generated)

| Endpoint | Description |
|----------|-------------|
| `/swagger/index.html` | Swagger UI (interactive docs) |
| `/swagger/doc.json` | OpenAPI spec (JSON) |
| `/health` | Health check |

### Message Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/topics/{topic}/messages` | POST | Publish a message |
| `/api/v1/topics/{topic}/messages` | GET | Consume messages |
| `/api/v1/topics/{topic}/subscribe` | POST | Subscribe to topic |
| `/api/v1/topics/{topic}/ack` | POST | Acknowledge messages |

### Admin Operations

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/admin/topics` | GET | List all topics |
| `/admin/topics/{topic}/stats` | GET | Get topic statistics |
| `/admin/topics/{topic}/messages` | GET | View messages |
| `/admin/topics/{topic}/messages` | DELETE | Purge all messages |
| `/admin/topics/{topic}/messages/{id}` | DELETE | Delete specific message |
| `/admin/topics/{topic}/consumers` | GET | List consumers |

## Build & Run

```bash
# Generate Swagger docs (auto-generated OpenAPI)
make swagger

# Build (includes swagger generation)
make build

# Run locally
make run

# Run tests
make test

# Build Docker image
make docker

# Run in Docker
make docker-run
```

## Auto-Generated OpenAPI

The OpenAPI specification is **auto-generated** from code annotations using [swaggo/swag](https://github.com/swaggo/swag).

- Swagger UI: http://localhost:8082/swagger/index.html
- JSON spec: http://localhost:8082/swagger/doc.json

Regenerate after code changes:
```bash
make swagger
```

## Example Usage

**Note:** HTTP admin APIs are on port **8082**. gRPC (for streamer/collector) is on port **8081**.

### Publish a message (via HTTP - for testing)
```bash
curl -X POST http://localhost:8082/api/v1/topics/gpu-telemetry/messages \
  -H "Content-Type: application/json" \
  -d '{"payload": {"metric": "gpu_util", "value": 85}}'
```

### Subscribe a consumer
```bash
curl -X POST http://localhost:8082/api/v1/topics/gpu-telemetry/subscribe \
  -H "Content-Type: application/json" \
  -d '{"consumer_id": "collector-1", "group": "collectors"}'
```

### Consume messages
```bash
curl "http://localhost:8082/api/v1/topics/gpu-telemetry/messages?consumer_id=collector-1&group=collectors&max_messages=10"
```

### Acknowledge messages
```bash
curl -X POST http://localhost:8082/api/v1/topics/gpu-telemetry/ack \
  -H "Content-Type: application/json" \
  -d '{"consumer_id": "collector-1", "message_ids": ["msg-id-1", "msg-id-2"]}'
```

### View topic stats (Admin)
```bash
curl http://localhost:8082/admin/topics/gpu-telemetry/stats
```

### View messages (Admin)
```bash
curl "http://localhost:8082/admin/topics/gpu-telemetry/messages?limit=10"
```

### Delete a message (Admin)
```bash
curl -X DELETE http://localhost:8082/admin/topics/gpu-telemetry/messages/msg-id-1
```

### Purge all messages (Admin)
```bash
curl -X DELETE http://localhost:8082/admin/topics/gpu-telemetry/messages
```
