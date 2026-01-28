# Telemetry Streamer Service Makefile
# ====================================
#
# This Makefile is self-contained for the Streamer microservice.
# Run all commands from the streamer/ directory.
#

.PHONY: all build test test-cover run run-mq docker clean help proto proto-install

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=gofmt
GOMOD=$(GOCMD) mod

# Proto parameters
PROTOC=protoc
PROTO_DIR=proto
PB_DIR=pkg/pb

# Service name
SERVICE_NAME=streamer

# Binary output
BIN_DIR=bin
BIN_NAME=$(BIN_DIR)/$(SERVICE_NAME)

# Docker
DOCKER_IMAGE=local/telemetry-$(SERVICE_NAME)
VERSION?=latest

# Data file path (relative to streamer folder)
DATA_FILE=./data/dcgm_metrics.csv

# Default target
all: proto fmt vet test build

## proto-install: Install protoc Go plugins (run once)
proto-install:
	@echo "Installing protoc Go plugins..."
	$(GOCMD) install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GOCMD) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "Make sure $(shell go env GOPATH)/bin is in your PATH"

## proto: Generate Go code from proto files (only if protoc is available)
proto:
	@if command -v $(PROTOC) >/dev/null 2>&1; then \
		echo "Generating protobuf code..."; \
		mkdir -p $(PB_DIR); \
		$(PROTOC) --go_out=$(PB_DIR) --go_opt=paths=source_relative \
			--go-grpc_out=$(PB_DIR) --go-grpc_opt=paths=source_relative \
			-I$(PROTO_DIR) \
			$(PROTO_DIR)/*.proto; \
		echo "Proto files generated in $(PB_DIR)"; \
	else \
		echo "protoc not found, skipping proto generation"; \
		if [ ! -f $(PB_DIR)/mq.pb.go ]; then \
			echo "ERROR: Proto files missing and protoc not installed."; \
			echo "Either install protoc (brew install protobuf && make proto-install)"; \
			echo "or ensure pkg/pb/*.pb.go files are committed to the repo."; \
			exit 1; \
		fi; \
	fi

## proto-check: Check if proto files exist (allows build without protoc if committed)
proto-check:
	@if [ ! -f $(PB_DIR)/mq.pb.go ]; then \
		echo "Proto files not found. Attempting to generate..."; \
		$(MAKE) proto; \
	fi

## build: Build the streamer binary
build: proto-check
	@echo "Building $(SERVICE_NAME)..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -ldflags="-w -s" -o $(BIN_NAME) ./cmd

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

## test-cover: Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## test-cover-report: Show coverage percentage
test-cover-report: test-cover
	@$(GOCMD) tool cover -func=coverage.out | tail -1

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

## mod-tidy: Tidy go modules
mod-tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy

## run: Run the streamer with MQ enabled (10 records every 5s via gRPC on port 8081)
run: build
	@echo "Running $(SERVICE_NAME) with MQ (gRPC)..."
	STREAMER_CSV_FILE_PATH=$(DATA_FILE) \
	STREAMER_MQ_BROKER_ADDR=localhost:8081 \
	STREAMER_MQ_ENABLED=true \
	STREAMER_LOG_LEVEL=info \
	STREAMER_STREAM_INTERVAL=5s \
	STREAMER_BATCH_SIZE=10 \
	STREAMER_FULL_FILE_BATCH=false \
	./$(BIN_NAME)

## dry-run: Run the streamer locally without MQ (console output only, for testing)
dry-run: build
	@echo "Running $(SERVICE_NAME) in dry-run mode (no MQ)..."
	STREAMER_CSV_FILE_PATH=$(DATA_FILE) \
	STREAMER_MQ_ENABLED=false \
	STREAMER_LOG_LEVEL=info \
	STREAMER_STREAM_INTERVAL=5s \
	STREAMER_BATCH_SIZE=10 \
	./$(BIN_NAME)

## run-single: Run the streamer sending 1 record per second (for testing)
run-single: build
	@echo "Running $(SERVICE_NAME) in single record mode..."
	STREAMER_CSV_FILE_PATH=$(DATA_FILE) \
	STREAMER_MQ_ENABLED=false \
	STREAMER_LOG_LEVEL=debug \
	STREAMER_STREAM_INTERVAL=1s \
	STREAMER_BATCH_SIZE=1 \
	./$(BIN_NAME)

## docker: Build Docker image (includes proto generation)
docker:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run --rm -v $(PWD)/data:/data $(DOCKER_IMAGE):latest

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html
	rm -rf $(PB_DIR)/*.pb.go

## help: Show this help message
help:
	@echo "Telemetry Streamer - Available targets:"
	@echo ""
	@echo "  proto-install   Install protoc Go plugins (run once)"
	@echo "  proto           Generate Go code from proto files"
	@echo "  build           Build the streamer binary"
	@echo "  test            Run tests"
	@echo "  test-cover      Run tests with coverage"
	@echo "  fmt             Format code"
	@echo "  vet             Run go vet"
	@echo "  mod-tidy        Tidy go modules"
	@echo "  run             Run locally (MQ disabled)"
	@echo "  run-mq          Run locally with MQ (gRPC)"
	@echo "  docker          Build Docker image"
	@echo "  docker-run      Run Docker container"
	@echo "  clean           Clean build artifacts"
