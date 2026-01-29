# Telemetry Streamer Dockerfile
# ==============================
#
# Multi-stage build with protoc for gRPC code generation.
# The final image is minimal and production-ready.
#
# Build: docker build -t local/telemetry-streamer .
# Run:   docker run -v /path/to/data:/data local/telemetry-streamer

# =============================================================================
# Stage 1: Build the Go binary
# =============================================================================
# If pkg/pb/*.pb.go files are committed, no proto generation needed.
# If not, install protoc and generate them.
FROM golang:1.23-alpine AS builder

# Install git and optionally protoc
RUN apk add --no-cache git ca-certificates protobuf protobuf-dev

# Install Go protoc plugins (only used if proto files need regeneration)
# Pin versions compatible with Go 1.23
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.35.2 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

WORKDIR /build

# Copy go mod files first for caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code (including pkg/pb if committed)
COPY . .

# Generate proto files only if they don't exist
RUN if [ ! -f pkg/pb/mq.pb.go ]; then \
        echo "Generating proto files..."; \
        mkdir -p pkg/pb && \
        protoc --go_out=pkg/pb --go_opt=paths=source_relative \
               --go-grpc_out=pkg/pb --go-grpc_opt=paths=source_relative \
               -Iproto \
               proto/*.proto; \
    else \
        echo "Using existing proto files from repo"; \
    fi

# Build the binary
# Build for native architecture (supports both amd64 and arm64)
RUN CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-w -s" -o streamer ./cmd

# =============================================================================
# Stage 3: Final minimal image
# =============================================================================
FROM alpine:3.19

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy the binary
COPY --from=builder /build/streamer .

# Create data directory and copy CSV file
RUN mkdir -p /data
COPY --from=builder /build/data/dcgm_metrics.csv /data/dcgm_metrics.csv
RUN chown -R appuser:appuser /app /data

# Switch to non-root user
USER appuser

# Set environment variables
# 10 records every 5 seconds, loops continuously via gRPC
ENV STREAMER_CSV_FILE_PATH=/data/dcgm_metrics.csv
ENV STREAMER_MQ_BROKER_ADDR=mq-broker:8081
ENV STREAMER_TOPIC=gpu-telemetry
ENV STREAMER_STREAM_INTERVAL=5s
ENV STREAMER_FULL_FILE_BATCH=false
ENV STREAMER_BATCH_SIZE=10
ENV STREAMER_MQ_ENABLED=true
ENV STREAMER_LOG_LEVEL=info

# Health check - the streamer doesn't expose HTTP, so we check the process
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD pgrep streamer || exit 1

# Run the streamer
CMD ["./streamer"]
