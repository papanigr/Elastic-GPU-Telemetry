# API Gateway Makefile

SERVICE_NAME := api-gateway
BIN_NAME := gateway
DOCKER_IMAGE := gpu-telemetry-gateway
DOCKER_TAG := latest

.PHONY: help build clean test test-coverage run dry-run docker-build docker-run lint fmt tidy swagger

## help: Show this help message
help:
	@echo "API Gateway - $(SERVICE_NAME)"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## swagger: Generate Swagger/OpenAPI documentation (auto-generated)
swagger:
	@echo "Generating Swagger documentation..."
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/main.go -o docs --parseDependency --parseInternal; \
	else \
		echo "swag not installed. Installing..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
		swag init -g cmd/main.go -o docs --parseDependency --parseInternal; \
	fi
	@echo "Swagger docs generated in docs/"

## build: Build the gateway binary (generates swagger first)
build: swagger
	@echo "Building $(SERVICE_NAME)..."
	go build -ldflags="-w -s" -o bin/$(BIN_NAME) ./cmd

## clean: Remove build artifacts and coverage files
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -testcache

## test: Run unit tests
test:
	@echo "Running tests..."
	go test -v -race ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## run: Run the gateway with PostgreSQL connection
run: build
	@echo "Running $(SERVICE_NAME)..."
	GATEWAY_HTTP_PORT=8080 \
	GATEWAY_DATABASE_URL="postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable" \
	GATEWAY_LOG_LEVEL=info \
	./bin/$(BIN_NAME)

## dry-run: Run without database (will fail health check but shows startup)
dry-run: build
	@echo "Running $(SERVICE_NAME) in dry-run mode..."
	@echo "Note: Database connection will fail, but you can see the startup logs"
	GATEWAY_HTTP_PORT=8080 \
	GATEWAY_DATABASE_URL="postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable" \
	GATEWAY_LOG_LEVEL=debug \
	./bin/$(BIN_NAME)

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run --rm -it \
		--network host \
		-e GATEWAY_DATABASE_URL="postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable" \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

## lint: Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, running go vet..."; \
		go vet ./...; \
	fi

## fmt: Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

## tidy: Tidy and verify dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy
	go mod verify

## verify: Run all checks (fmt, lint, test)
verify: fmt lint test
	@echo "All checks passed!"
