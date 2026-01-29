# Telemetry Streamer Service

The Telemetry Streamer reads GPU telemetry data from a CSV file and streams it to the custom message queue via **gRPC**.

## Overview

This service simulates real-time GPU metrics by:
1. Reading telemetry data from a CSV file in a continuous loop
2. Updating timestamps to the current time for each record
3. Publishing records to the MQ broker via **gRPC (port 8081)**

## Architecture

```
┌─────────────────┐          gRPC (port 8081)          ┌─────────────────┐
│    Streamer     │ ──────────────────────────────────▶│    MQ Broker    │
│                 │                                     │                 │
│  CSV → Records  │         pb.MQServiceClient          │  Topic Queue    │
└─────────────────┘                                     └─────────────────┘
```

## Project Structure

```
streamer/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── config.go            # Configuration management
│   ├── csv_reader.go        # CSV file reader
│   ├── streamer.go          # Main streaming logic
├── pkg/
│   ├── models/
│   │   └── telemetry.go     # Data models
│   ├── mq/
│   │   └── client/
│   │       ├── client.go    # HTTP MQ client (fallback)
│   │       └── grpc_client.go  # gRPC MQ client (primary)
│   └── pb/                  # Generated protobuf code
│       ├── mq.pb.go
│       └── mq_grpc.pb.go
├── proto/
│   └── mq.proto             # Protobuf definitions
├── data/
│   └── dcgm_metrics.csv     # Sample telemetry data
├── Dockerfile               # Container build (includes protoc)
├── Makefile                 # Build automation
├── go.mod                   # Go module
└── README.md                # This file
```

## Configuration

The service is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `STREAMER_CSV_FILE_PATH` | `/data/dcgm_metrics.csv` | Path to CSV file |
| `STREAMER_MQ_BROKER_ADDR` | `mq-broker:8081` | MQ broker gRPC address |
| `STREAMER_TOPIC` | `gpu-telemetry` | Topic to publish to |
| `STREAMER_STREAM_INTERVAL` | `5s` | Interval between batches |
| `STREAMER_BATCH_SIZE` | `10` | Records per batch |
| `STREAMER_FULL_FILE_BATCH` | `false` | Send entire file as one batch |
| `STREAMER_MQ_ENABLED` | `true` | Enable/disable MQ publishing |
| `STREAMER_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |

## Prerequisites

### If proto files are committed (default):
No additional setup needed - just build and run.

### If you need to regenerate proto files:
```bash
# Install protoc (macOS)
brew install protobuf

# Install Go plugins (run once)
make proto-install

# Generate proto files
make proto
```

## Build & Run

All commands should be run from the `streamer/` directory.

### Local Development

```bash
# Build (uses committed proto files)
make build

# Run with MQ enabled (requires MQ broker on localhost:8081)
make run

# Dry run (MQ disabled - for testing CSV reading, console output only)
make dry-run

# Run tests
make test

# Run tests with coverage
make test-cover
```

### With MQ Broker

```bash
# Terminal 1: Start MQ broker
cd ../mq && make run

# Terminal 2: Start streamer
cd ../streamer && make run
```

### Docker

```bash
# Build Docker image
make docker

# Run in Docker
make docker-run

# Or manually:
docker run -v /path/to/data:/data local/telemetry-streamer:latest
```

## Data Format

The service expects a CSV file with the following columns:

| Column | Description |
|--------|-------------|
| `timestamp` | Original timestamp (replaced with current time) |
| `metric_name` | DCGM metric name (e.g., DCGM_FI_DEV_GPU_UTIL) |
| `gpu_id` | GPU index on the host |
| `device` | Device name (e.g., nvidia0) |
| `uuid` | GPU UUID |
| `modelName` | GPU model name |
| `Hostname` | Host where the GPU is located |
| `value` | Metric value |
| `labels_raw` | Raw labels string |

## gRPC Communication

The streamer uses gRPC to communicate with the MQ broker:

```protobuf
service MQService {
  rpc Publish(PublishRequest) returns (PublishResponse);
}

message PublishRequest {
  string topic = 1;
  string id = 2;
  bytes payload = 3;    // JSON-encoded TelemetryRecord
  google.protobuf.Timestamp timestamp = 4;
}
```

The `pkg/pb/` directory contains generated Go code from `proto/mq.proto`.

## Scaling

When deployed to Kubernetes, multiple instances can run simultaneously:
- Each reads from the same CSV file (mounted as shared volume)
- Each publishes to the same topic via gRPC
- MQ broker handles concurrent connections from all streamers

## Makefile Targets

Run `make help` to see all available targets. Uses podman or docker automatically.

| Command | Description | Requirements |
|---------|-------------|--------------|
| `make help` | Show all available targets | - |
| **Build** | | |
| `make build` | Build the streamer binary to `bin/streamer` | - |
| `make clean` | Remove binary, proto files, and coverage files | - |
| `make all` | Run proto, fmt, vet, test, and build | - |
| **Testing** | | |
| `make test` | Run all unit tests | - |
| `make test-cover` | Run tests with coverage (generates `coverage.html`) | - |
| `make test-cover-report` | Show coverage percentage | - |
| **Code Quality** | | |
| `make fmt` | Format code with gofmt | - |
| `make vet` | Run go vet | - |
| `make mod-tidy` | Tidy Go modules (`go mod tidy`) | - |
| **Proto** | | |
| `make proto` | Generate Go code from `.proto` files | `protoc` installed |
| `make proto-install` | Install protoc Go plugins | Go installed |
| `make proto-check` | Check if proto files exist, generate if missing | - |
| **Run** | | |
| `make run` | Run with MQ enabled (10 records/5s via gRPC) | MQ broker on `:8081` |
| `make dry-run` | Run without MQ (console output only) | - |
| `make run-single` | Run sending 1 record/second (debug mode) | - |
| **Docker** | | |
| `make docker` | Build Docker image (uses podman or docker) | Container runtime |
| `make docker-run` | Run container with data volume | Container runtime |
