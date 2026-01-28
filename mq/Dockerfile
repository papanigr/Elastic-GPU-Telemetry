# MQ Broker Dockerfile
# =====================
#
# Multi-stage build with protoc for gRPC code generation.
# The final image is minimal and production-ready.
#
# Build: docker build -t local/mq-broker .
# Run:   docker run -p 8081:8081 -p 8082:8082 local/mq-broker

# =============================================================================
# Stage 1: Build the Go binary
# =============================================================================
# If pkg/pb/*.pb.go files are committed, no proto generation needed.
# If not, install protoc and generate them.
FROM golang:1.22-alpine AS builder

# Install git and optionally protoc
RUN apk add --no-cache git ca-certificates protobuf protobuf-dev

# Install Go protoc plugins (only used if proto files need regeneration)
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o mq-broker ./cmd

# =============================================================================
# Stage 3: Final minimal image
# =============================================================================
FROM alpine:3.19

# Install ca-certificates for HTTPS and wget for healthcheck
RUN apk --no-cache add ca-certificates tzdata wget

# Create non-root user
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy the binary
COPY --from=builder /build/mq-broker .

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Set environment variables
ENV MQ_GRPC_PORT=8081
ENV MQ_HTTP_PORT=8082
ENV MQ_MAX_QUEUE_SIZE=10000
ENV MQ_MAX_MESSAGE_AGE=5m
ENV MQ_ACK_TIMEOUT=30s
ENV MQ_CLEANUP_INTERVAL=10s
ENV MQ_LOG_LEVEL=info

# Expose ports (gRPC and HTTP admin)
EXPOSE 8081 8082

# Health check (uses HTTP admin port)
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8082/health || exit 1

# Run the broker
CMD ["./mq-broker"]
