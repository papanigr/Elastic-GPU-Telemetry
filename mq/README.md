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

## REST API Reference

All HTTP endpoints are served on port **8082** by default.

### Core Endpoints

| Method | Endpoint | Description | Example |
|--------|----------|-------------|---------|
| GET | `/health` | Health check | `curl http://localhost:8082/health` |
| GET | `/swagger/index.html` | Swagger UI (interactive docs) | Open in browser |
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

