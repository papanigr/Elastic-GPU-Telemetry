# Telemetry Streamer Service Makefile
# ====================================
#
# This Makefile is self-contained for the Streamer microservice.
# Run all commands from the streamer/ directory.
#

.PHONY: all build test test-cover run run-mq docker clean help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=gofmt
GOMOD=$(GOCMD) mod

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
all: fmt vet test build

## build: Build the streamer binary
build:
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

## run: Run the streamer locally (MQ disabled)
run: build
	@echo "Running $(SERVICE_NAME)..."
	STREAMER_CSV_FILE_PATH=$(DATA_FILE) \
	STREAMER_MQ_ENABLED=false \
	STREAMER_LOG_LEVEL=debug \
	./$(BIN_NAME)

## run-mq: Run the streamer with MQ enabled
run-mq: build
	@echo "Running $(SERVICE_NAME) with MQ..."
	STREAMER_CSV_FILE_PATH=$(DATA_FILE) \
	STREAMER_MQ_BROKER_URL=http://localhost:8081 \
	STREAMER_MQ_ENABLED=true \
	STREAMER_LOG_LEVEL=debug \
	./$(BIN_NAME)

## docker: Build Docker image
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

## help: Show this help message
help:
	@echo "Telemetry Streamer - Available targets:"
	@echo ""
	@echo "  build           Build the streamer binary"
	@echo "  test            Run tests"
	@echo "  test-cover      Run tests with coverage"
	@echo "  fmt             Format code"
	@echo "  vet             Run go vet"
	@echo "  mod-tidy        Tidy go modules"
	@echo "  run             Run locally (MQ disabled)"
	@echo "  run-mq          Run locally (MQ enabled)"
	@echo "  docker          Build Docker image"
	@echo "  docker-run      Run Docker container"
	@echo "  clean           Clean build artifacts"
