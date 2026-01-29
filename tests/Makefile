# Integration Tests Makefile
# Works with both Docker and Podman - no compose needed!

# Auto-detect container engine (prefer podman, fallback to docker)
ENGINE := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
ENGINE_NAME := $(notdir $(ENGINE))

# Container names
POSTGRES_CONTAINER := test-postgres
MQ_CONTAINER := test-mq
COLLECTOR_CONTAINER := test-collector
GATEWAY_CONTAINER := test-gateway
STREAMER_CONTAINER := test-streamer

# Network name
NETWORK_NAME := test-network

# Image names (will be built from source)
MQ_IMAGE := gpu-telemetry-mq:test
COLLECTOR_IMAGE := gpu-telemetry-collector:test
GATEWAY_IMAGE := gpu-telemetry-gateway:test
STREAMER_IMAGE := gpu-telemetry-streamer:test

# Ports (different from dev to avoid conflicts)
# MQ internal ports: gRPC=8081, HTTP=8082 (per mq/internal/config/config.go)
POSTGRES_PORT := 5433
MQ_HTTP_PORT := 8083
MQ_GRPC_PORT := 8084
GATEWAY_PORT := 8085

# Database config
POSTGRES_DB := telemetry_test
POSTGRES_USER := postgres
POSTGRES_PASSWORD := postgres

.PHONY: help check-engine build up down logs test clean deps status health

## help: Show this help message
help:
	@echo "Integration Tests"
	@echo ""
	@echo "Container Engine: $(ENGINE_NAME)"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## check-engine: Verify container engine is available
check-engine:
	@if [ -z "$(ENGINE)" ]; then \
		echo "Error: No container engine found!"; \
		echo "Please install Docker or Podman:"; \
		echo "  - Docker: https://docs.docker.com/get-docker/"; \
		echo "  - Podman: brew install podman"; \
		exit 1; \
	fi
	@echo "Using container engine: $(ENGINE_NAME)"

## deps: Install test dependencies
deps:
	@echo "Installing test dependencies..."
	go mod tidy

## network: Create test network
network: check-engine
	@$(ENGINE) network inspect $(NETWORK_NAME) >/dev/null 2>&1 || \
		$(ENGINE) network create $(NETWORK_NAME)
	@echo "Network $(NETWORK_NAME) ready"

## build: Build all service images
build: check-engine
	@echo "Building service images..."
	@echo "Building MQ..."
	@cd ../mq && $(ENGINE) build -t $(MQ_IMAGE) .
	@echo "Building Collector..."
	@cd ../collector && $(ENGINE) build -t $(COLLECTOR_IMAGE) .
	@echo "Building Gateway..."
	@cd ../gateway && $(ENGINE) build -t $(GATEWAY_IMAGE) .
	@echo "Building Streamer..."
	@cd ../streamer && $(ENGINE) build -t $(STREAMER_IMAGE) .
	@echo "All images built!"

## up-postgres: Start PostgreSQL container
up-postgres: check-engine network
	@echo "Starting PostgreSQL..."
	@if $(ENGINE) ps -a --format '{{.Names}}' | grep -q "^$(POSTGRES_CONTAINER)$$"; then \
		$(ENGINE) start $(POSTGRES_CONTAINER) 2>/dev/null || true; \
	else \
		$(ENGINE) run -d \
			--name $(POSTGRES_CONTAINER) \
			--network $(NETWORK_NAME) \
			-e POSTGRES_DB=$(POSTGRES_DB) \
			-e POSTGRES_USER=$(POSTGRES_USER) \
			-e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) \
			-p $(POSTGRES_PORT):5432 \
			-v $(PWD)/../db/init/init.sql:/docker-entrypoint-initdb.d/01_init.sql:ro \
			-v $(PWD)/../db/migrations:/docker-entrypoint-initdb.d/migrations:ro \
			--health-cmd="pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB)" \
			--health-interval=5s \
			--health-timeout=5s \
			--health-retries=10 \
			postgres:15-alpine; \
	fi
	@echo "PostgreSQL starting on localhost:$(POSTGRES_PORT)"

## up-mq: Start MQ container
up-mq: check-engine network build
	@echo "Starting MQ..."
	@$(ENGINE) rm -f $(MQ_CONTAINER) 2>/dev/null || true
	@$(ENGINE) run -d \
		--name $(MQ_CONTAINER) \
		--network $(NETWORK_NAME) \
		-e MQ_GRPC_PORT=8081 \
		-e MQ_HTTP_PORT=8082 \
		-e MQ_MAX_QUEUE_SIZE=10000 \
		-e MQ_LOG_LEVEL=debug \
		-p $(MQ_GRPC_PORT):8081 \
		-p $(MQ_HTTP_PORT):8082 \
		$(MQ_IMAGE)
	@echo "MQ starting on localhost:$(MQ_HTTP_PORT) (HTTP) and localhost:$(MQ_GRPC_PORT) (gRPC)"

## up-collector: Start Collector container
up-collector: check-engine network
	@echo "Starting Collector..."
	@$(ENGINE) rm -f $(COLLECTOR_CONTAINER) 2>/dev/null || true
	@$(ENGINE) run -d \
		--name $(COLLECTOR_CONTAINER) \
		--network $(NETWORK_NAME) \
		-e COLLECTOR_MQ_BROKER_ADDR=$(MQ_CONTAINER):8081 \
		-e COLLECTOR_DATABASE_URL="postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_CONTAINER):5432/$(POSTGRES_DB)?sslmode=disable" \
		-e COLLECTOR_DB_ENABLED=true \
		-e COLLECTOR_TOPIC=gpu-telemetry \
		-e COLLECTOR_CONSUMER_GROUP=telemetry-collectors \
		-e COLLECTOR_CONSUMER_ID=collector-1 \
		-e COLLECTOR_NUM_WORKERS=3 \
		-e COLLECTOR_BATCH_SIZE=10 \
		-e COLLECTOR_POLL_INTERVAL=1s \
		-e COLLECTOR_LOG_LEVEL=debug \
		$(COLLECTOR_IMAGE)
	@echo "Collector starting..."

## up-gateway: Start Gateway container
up-gateway: check-engine network
	@echo "Starting Gateway..."
	@$(ENGINE) rm -f $(GATEWAY_CONTAINER) 2>/dev/null || true
	@$(ENGINE) run -d \
		--name $(GATEWAY_CONTAINER) \
		--network $(NETWORK_NAME) \
		-e GATEWAY_HTTP_PORT=8080 \
		-e GATEWAY_DATABASE_URL="postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_CONTAINER):5432/$(POSTGRES_DB)?sslmode=disable" \
		-e GATEWAY_LOG_LEVEL=debug \
		-p $(GATEWAY_PORT):8080 \
		$(GATEWAY_IMAGE)
	@echo "Gateway starting on localhost:$(GATEWAY_PORT)"

## up: Start all test services
up: up-postgres up-mq
	@echo "Waiting for PostgreSQL to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if $(ENGINE) exec $(POSTGRES_CONTAINER) pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB) 2>/dev/null; then \
			echo "PostgreSQL is ready!"; \
			break; \
		fi; \
		echo "Waiting... ($$i/10)"; \
		sleep 2; \
	done
	@echo "Waiting for MQ to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s http://localhost:$(MQ_HTTP_PORT)/health >/dev/null 2>&1; then \
			echo "MQ is ready!"; \
			break; \
		fi; \
		echo "Waiting for MQ... ($$i/10)"; \
		sleep 2; \
	done
	@$(MAKE) up-collector
	@$(MAKE) up-gateway
	@echo ""
	@echo "Waiting for services to initialize..."
	@sleep 5
	@echo ""
	@echo "Test environment is ready!"
	@echo "  Gateway:  http://localhost:$(GATEWAY_PORT)"
	@echo "  MQ HTTP:  http://localhost:$(MQ_HTTP_PORT)"
	@echo "  MQ gRPC:  localhost:$(MQ_GRPC_PORT)"
	@echo "  Postgres: localhost:$(POSTGRES_PORT)"

## down: Stop all test services
down: check-engine
	@echo "Stopping test services..."
	@$(ENGINE) stop $(STREAMER_CONTAINER) 2>/dev/null || true
	@$(ENGINE) rm -f $(STREAMER_CONTAINER) 2>/dev/null || true
	@$(ENGINE) stop $(GATEWAY_CONTAINER) 2>/dev/null || true
	@$(ENGINE) rm -f $(GATEWAY_CONTAINER) 2>/dev/null || true
	@$(ENGINE) stop $(COLLECTOR_CONTAINER) 2>/dev/null || true
	@$(ENGINE) rm -f $(COLLECTOR_CONTAINER) 2>/dev/null || true
	@$(ENGINE) stop $(MQ_CONTAINER) 2>/dev/null || true
	@$(ENGINE) rm -f $(MQ_CONTAINER) 2>/dev/null || true
	@$(ENGINE) stop $(POSTGRES_CONTAINER) 2>/dev/null || true
	@$(ENGINE) rm -f $(POSTGRES_CONTAINER) 2>/dev/null || true
	@echo "Test services stopped."

## logs: Show logs from all services
logs: check-engine
	@echo "=== MQ Logs ===" && $(ENGINE) logs $(MQ_CONTAINER) 2>/dev/null | tail -20 || true
	@echo ""
	@echo "=== Collector Logs ===" && $(ENGINE) logs $(COLLECTOR_CONTAINER) 2>/dev/null | tail -20 || true
	@echo ""
	@echo "=== Gateway Logs ===" && $(ENGINE) logs $(GATEWAY_CONTAINER) 2>/dev/null | tail -20 || true

## logs-mq: Show MQ logs
logs-mq: check-engine
	$(ENGINE) logs -f $(MQ_CONTAINER)

## logs-collector: Show Collector logs
logs-collector: check-engine
	$(ENGINE) logs -f $(COLLECTOR_CONTAINER)

## logs-gateway: Show Gateway logs
logs-gateway: check-engine
	$(ENGINE) logs -f $(GATEWAY_CONTAINER)

## status: Show status of test services
status: check-engine
	@echo "Test Service Status:"
	@echo ""
	@$(ENGINE) ps -a --filter "name=test-" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

## health: Check health of all services
health: check-engine
	@echo "Checking service health..."
	@echo ""
	@echo "PostgreSQL (localhost:$(POSTGRES_PORT)):"
	@$(ENGINE) exec $(POSTGRES_CONTAINER) pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB) 2>/dev/null || echo "  Not available"
	@echo ""
	@echo "MQ HTTP (localhost:$(MQ_HTTP_PORT)):"
	@curl -s http://localhost:$(MQ_HTTP_PORT)/health | jq . 2>/dev/null || echo "  Not available"
	@echo ""
	@echo "Gateway (localhost:$(GATEWAY_PORT)):"
	@curl -s http://localhost:$(GATEWAY_PORT)/health | jq . 2>/dev/null || echo "  Not available"
	@echo ""
	@echo "Collector:"
	@$(ENGINE) logs --tail 5 $(COLLECTOR_CONTAINER) 2>/dev/null || echo "  Not running"

## test: Run all integration tests
test: deps
	@echo "Running integration tests..."
	@echo "Make sure the test environment is running (make up)"
	go test -tags=integration -v -timeout 5m ./integration/...

## test-short: Run integration tests in short mode
test-short: deps
	@echo "Running integration tests (short mode)..."
	go test -tags=integration -v -short -timeout 2m ./integration/...

## test-e2e: Run only end-to-end tests
test-e2e: deps
	@echo "Running end-to-end tests..."
	go test -tags=integration -v -timeout 3m -run "TestEndToEnd" ./integration/...

## test-api: Run only API tests
test-api: deps
	@echo "Running API tests..."
	go test -tags=integration -v -timeout 3m -run "Test.*Endpoint|Test.*Health|Test.*GPU" ./integration/...

## test-mq: Run only MQ tests
test-mq: deps
	@echo "Running MQ tests..."
	go test -tags=integration -v -timeout 3m -run "TestMQ" ./integration/...

## test-scaling: Run only scaling tests
test-scaling: deps
	@echo "Running scaling tests..."
	go test -tags=integration -v -timeout 5m -run "Test.*Concurrent|Test.*Scaled|Test.*Volume" ./integration/...

## test-all: Start environment, run tests, stop environment
test-all: up
	@echo "Running all integration tests..."
	@sleep 5
	go test -tags=integration -v -timeout 5m ./integration/... || ($(MAKE) down && exit 1)
	@$(MAKE) down
	@echo "All tests completed!"

## clean: Clean up test artifacts and containers
clean: down
	@echo "Cleaning up..."
	go clean -testcache
	rm -f coverage.out
	@$(ENGINE) network rm $(NETWORK_NAME) 2>/dev/null || true
	@echo "Clean complete."
