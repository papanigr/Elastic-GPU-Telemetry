# MQ Broker Service Makefile
# ===========================
#
# This Makefile is self-contained for the MQ Broker microservice.
# Run all commands from the mq/ directory.
#

.PHONY: all build test run docker clean help proto proto-install swagger

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
SERVICE_NAME=mq-broker

# Binary output
BIN_DIR=bin
BIN_NAME=$(BIN_DIR)/$(SERVICE_NAME)

# Docker
DOCKER_IMAGE=local/$(SERVICE_NAME)
VERSION?=latest

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

## swagger: Generate Swagger/OpenAPI documentation (auto-generated)
swagger:
	@echo "Generating Swagger documentation..."
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/main.go -o docs --parseDependency --parseInternal; \
	else \
		echo "swag not installed. Installing..."; \
		$(GOCMD) install github.com/swaggo/swag/cmd/swag@latest; \
		swag init -g cmd/main.go -o docs --parseDependency --parseInternal; \
	fi
	@echo "Swagger docs generated in docs/"

## build: Build the MQ broker binary (generates swagger first)
build: proto-check swagger
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

## run: Run the MQ broker locally
run: build
	@echo "Running $(SERVICE_NAME)..."
	MQ_GRPC_PORT=8081 \
	MQ_HTTP_PORT=8082 \
	MQ_LOG_LEVEL=debug \
	MQ_MAX_QUEUE_SIZE=10000 \
	./$(BIN_NAME)

## docker: Build Docker image (includes proto generation)
docker:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest .

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run --rm -p 8081:8081 -p 8082:8082 $(DOCKER_IMAGE):latest

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html
	rm -rf $(PB_DIR)/*.pb.go

## help: Show this help message
help:
	@echo "MQ Broker - Available targets:"
	@echo ""
	@echo "  proto-install   Install protoc Go plugins (run once)"
	@echo "  proto           Generate Go code from proto files"
	@echo "  swagger         Generate Swagger/OpenAPI documentation"
	@echo "  build           Build the broker binary"
	@echo "  test            Run tests"
	@echo "  test-cover      Run tests with coverage"
	@echo "  fmt             Format code"
	@echo "  vet             Run go vet"
	@echo "  mod-tidy        Tidy go modules"
	@echo "  run             Run locally"
	@echo "  docker          Build Docker image"
	@echo "  docker-run      Run Docker container"
	@echo "  clean           Clean build artifacts"
