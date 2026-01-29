# Telemetry Collector Dockerfile
# Multi-stage build for minimal image size

# =============================================================================
# Stage 1: Builder
# =============================================================================
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy proto files and generated code
COPY proto/ proto/
COPY pkg/ pkg/

# Check if generated proto files exist, if not generate them
RUN if [ ! -f pkg/pb/mq.pb.go ]; then \
        echo "Proto files not found, installing protoc..."; \
        apk add --no-cache protobuf protobuf-dev; \
        go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.35.2; \
        go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1; \
        mkdir -p pkg/pb; \
        protoc --go_out=pkg/pb --go_opt=paths=source_relative \
               --go-grpc_out=pkg/pb --go-grpc_opt=paths=source_relative \
               -Iproto proto/*.proto; \
    else \
        echo "Using pre-generated proto files"; \
    fi

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# Build the binary
# Build for native architecture (supports both amd64 and arm64)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/collector ./cmd

# =============================================================================
# Stage 2: Runtime
# =============================================================================
FROM alpine:3.19

# Install ca-certificates for HTTPS and tzdata for timezone
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' collector

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/collector .

# Set ownership
RUN chown -R collector:collector /app

# Switch to non-root user
USER collector

# Environment variables with defaults
ENV COLLECTOR_MQ_BROKER_ADDR=mq-broker:8081
ENV COLLECTOR_TOPIC=gpu-telemetry
ENV COLLECTOR_CONSUMER_GROUP=telemetry-collectors
ENV COLLECTOR_BATCH_SIZE=20
ENV COLLECTOR_POLL_INTERVAL=500ms
ENV COLLECTOR_NUM_WORKERS=3
ENV COLLECTOR_DB_ENABLED=false
ENV COLLECTOR_DATABASE_URL=postgres://postgres:postgres@postgres:5432/telemetry?sslmode=disable
ENV COLLECTOR_LOG_LEVEL=info

# Health check - collector doesn't expose HTTP, so we just check if process is running
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD pgrep collector || exit 1

# Run the collector
ENTRYPOINT ["./collector"]
