# Collector Makefile

SERVICE_NAME := telemetry-collector
BIN_NAME := bin/collector
DOCKER_IMAGE := telemetry-collector
DOCKER_TAG := latest

# Go commands
GO := go
GOBUILD := $(GO) build
GOTEST := $(GO) test
GOMOD := $(GO) mod

# Proto
PROTOC := protoc
PROTO_DIR := proto
PB_DIR := pkg/pb

.PHONY: help build clean test run dry-run docker docker-run proto proto-install proto-check mod-tidy lint

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build the collector binary
build: proto-check
	@echo "Building $(SERVICE_NAME)..."
	$(GOBUILD) -ldflags="-w -s" -o $(BIN_NAME) ./cmd

## clean: Remove built binary, generated files, and coverage files
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f $(PB_DIR)/*.pb.go
	rm -f coverage.out coverage.html

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-cover: Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -cover -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

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

## proto-install: Install protoc Go plugins
proto-install:
	@echo "Installing protoc-gen-go and protoc-gen-go-grpc..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

## proto-check: Check if proto files exist (allows build without protoc if committed)
proto-check:
	@if [ ! -f $(PB_DIR)/mq.pb.go ]; then \
		echo "Proto files not found. Attempting to generate..."; \
		$(MAKE) proto; \
	fi

## mod-tidy: Tidy go modules
mod-tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy

## run: Run the collector with MQ + PostgreSQL (production mode)
run: build
	@echo "Running $(SERVICE_NAME) with MQ + PostgreSQL..."
	COLLECTOR_MQ_BROKER_ADDR=localhost:8081 \
	COLLECTOR_TOPIC=gpu-telemetry \
	COLLECTOR_CONSUMER_GROUP=telemetry-collectors \
	COLLECTOR_BATCH_SIZE=20 \
	COLLECTOR_POLL_INTERVAL=500ms \
	COLLECTOR_NUM_WORKERS=3 \
	COLLECTOR_DB_ENABLED=true \
	COLLECTOR_DATABASE_URL="postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable" \
	COLLECTOR_LOG_LEVEL=info \
	./$(BIN_NAME)

## dry-run: Run without PostgreSQL (console output only, for testing)
dry-run: build
	@echo "Running $(SERVICE_NAME) in dry-run mode (no DB)..."
	COLLECTOR_MQ_BROKER_ADDR=localhost:8081 \
	COLLECTOR_TOPIC=gpu-telemetry \
	COLLECTOR_CONSUMER_GROUP=telemetry-collectors \
	COLLECTOR_BATCH_SIZE=10 \
	COLLECTOR_POLL_INTERVAL=1s \
	COLLECTOR_NUM_WORKERS=2 \
	COLLECTOR_DB_ENABLED=false \
	COLLECTOR_LOG_LEVEL=info \
	./$(BIN_NAME)

## run-high: Run with high throughput settings (for 10 streamers)
run-high: build
	@echo "Running $(SERVICE_NAME) in high-throughput mode..."
	COLLECTOR_MQ_BROKER_ADDR=localhost:8081 \
	COLLECTOR_TOPIC=gpu-telemetry \
	COLLECTOR_CONSUMER_GROUP=telemetry-collectors \
	COLLECTOR_BATCH_SIZE=50 \
	COLLECTOR_POLL_INTERVAL=200ms \
	COLLECTOR_NUM_WORKERS=5 \
	COLLECTOR_DB_ENABLED=true \
	COLLECTOR_DATABASE_URL="postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable" \
	COLLECTOR_LOG_LEVEL=info \
	./$(BIN_NAME)

## docker: Build Docker image
docker:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

## docker-run: Run Docker container with PostgreSQL
docker-run:
	@echo "Running Docker container..."
	docker run --rm -it \
		--network host \
		-e COLLECTOR_MQ_BROKER_ADDR=localhost:8081 \
		-e COLLECTOR_DB_ENABLED=true \
		-e COLLECTOR_DATABASE_URL="postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable" \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

## lint: Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...
