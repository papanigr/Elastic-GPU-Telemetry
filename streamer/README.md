# Telemetry Streamer Service

The Telemetry Streamer reads GPU telemetry data from a CSV file and streams it to the custom message queue.

## Overview

This service simulates real-time GPU metrics by:
1. Reading telemetry data from a CSV file in a continuous loop
2. Updating timestamps to the current time for each record
3. Publishing records to the message queue

## Project Structure

```
streamer/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── config.go            # Configuration management
│   ├── csv_reader.go        # CSV file reader
│   ├── csv_reader_test.go   # CSV reader tests
│   ├── streamer.go          # Main streaming logic
│   └── streamer_test.go     # Streamer tests
├── pkg/
│   ├── models/
│   │   └── telemetry.go     # Data models
│   └── mq/
│       └── client/
│           └── client.go    # MQ client
├── bin/                     # Build output (gitignored)
├── Dockerfile               # Container build
├── Makefile                 # Build automation
├── go.mod                   # Go module
├── go.sum                   # Dependencies
└── README.md                # This file
```

## Configuration

The service is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `STREAMER_CSV_FILE_PATH` | `/data/dcgm_metrics.csv` | Path to CSV file |
| `STREAMER_MQ_BROKER_URL` | `http://mq-broker:8081` | MQ broker URL |
| `STREAMER_TOPIC` | `gpu-telemetry` | Topic to publish to |
| `STREAMER_STREAM_INTERVAL` | `100ms` | Interval between records |
| `STREAMER_BATCH_SIZE` | `1` | Records per batch |
| `STREAMER_MQ_ENABLED` | `true` | Enable/disable MQ |
| `STREAMER_LOG_LEVEL` | `info` | Log level |

## Build & Run

All commands should be run from the `streamer/` directory.

### Local Development

```bash
# Build
make build

# Run (MQ disabled - for testing)
make run

# Run with MQ enabled
make run-mq

# Run tests
make test

# Run tests with coverage
make test-cover

# Format code
make fmt

# Tidy modules
make mod-tidy
```

### Docker

```bash
# Build Docker image
make docker

# Run in Docker (mounts data from parent directory)
make docker-run

# Or manually:
docker run -v /path/to/data:/data local/telemetry-streamer:latest
```

## Data Format

The service expects a CSV file with the following columns:

| Column | Description |
|--------|-------------|
| `timestamp` | Original timestamp (ignored, replaced with current time) |
| `metric_name` | DCGM metric name (e.g., DCGM_FI_DEV_GPU_UTIL) |
| `gpu_id` | GPU index on the host |
| `device` | Device name (e.g., nvidia0) |
| `uuid` | GPU UUID |
| `modelName` | GPU model name |
| `Hostname` | Host where the GPU is located |
| `container` | Container name (optional) |
| `pod` | Kubernetes pod name (optional) |
| `namespace` | Kubernetes namespace (optional) |
| `value` | Metric value |
| `labels_raw` | Raw labels string |

## Scaling

When deployed to Kubernetes, multiple instances of this service can run simultaneously. Each instance:
- Reads from the same CSV file (mounted as a shared volume)
- Publishes to the same topic
- Has its own position in the file (independent streaming)

This enables horizontal scaling of the data ingestion pipeline.

## Makefile Targets

```
make help
```

| Target | Description |
|--------|-------------|
| `build` | Build the streamer binary |
| `test` | Run tests |
| `test-cover` | Run tests with coverage |
| `fmt` | Format code |
| `vet` | Run go vet |
| `mod-tidy` | Tidy go modules |
| `run` | Run locally (MQ disabled) |
| `run-mq` | Run locally (MQ enabled) |
| `docker` | Build Docker image |
| `docker-run` | Run Docker container |
| `clean` | Clean build artifacts |
