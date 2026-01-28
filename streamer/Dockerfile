# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /bin/streamer \
    ./cmd

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -g '' appuser

# Create data directory
RUN mkdir -p /data && chown appuser:appuser /data

# Copy binary from builder
COPY --from=builder /bin/streamer /bin/streamer

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Use non-root user
USER appuser

# Set environment variables
ENV STREAMER_CSV_FILE_PATH=/data/dcgm_metrics.csv
ENV STREAMER_MQ_BROKER_URL=http://mq-broker:8081
ENV STREAMER_TOPIC=gpu-telemetry
ENV STREAMER_STREAM_INTERVAL=100ms
ENV STREAMER_BATCH_SIZE=1
ENV STREAMER_MQ_ENABLED=true
ENV STREAMER_LOG_LEVEL=info

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD pgrep streamer || exit 1

# Run the binary
ENTRYPOINT ["/bin/streamer"]
