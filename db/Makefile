# PostgreSQL Database Makefile
# Works with both Docker and Podman - no compose needed!

# Auto-detect container engine (prefer podman, fallback to docker)
ENGINE := $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
ENGINE_NAME := $(notdir $(ENGINE))

# Container and image names
CONTAINER_NAME := gpu-telemetry-db
IMAGE_NAME := gpu-telemetry-db
IMAGE_TAG := latest
VOLUME_NAME := gpu-telemetry-pgdata

# Database configuration
POSTGRES_DB := telemetry
POSTGRES_USER := postgres
POSTGRES_PASSWORD := postgres
POSTGRES_PORT := 5432

.PHONY: help check-engine build up down logs shell psql status clean migrate tables describe count sample gpus rows all tail export restart

## help: Show this help message
help:
	@echo "GPU Telemetry PostgreSQL Database"
	@echo ""
	@echo "Container Engine: $(ENGINE_NAME)"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Lifecycle:"
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

## build: Build the PostgreSQL Docker image
build: check-engine
	@echo "Building PostgreSQL image..."
	$(ENGINE) build -t $(IMAGE_NAME):$(IMAGE_TAG) .
	@echo "Image built: $(IMAGE_NAME):$(IMAGE_TAG)"

## up: Start PostgreSQL container
up: check-engine
	@echo "Starting PostgreSQL with $(ENGINE_NAME)..."
	@# Create volume if it doesn't exist
	@$(ENGINE) volume inspect $(VOLUME_NAME) >/dev/null 2>&1 || $(ENGINE) volume create $(VOLUME_NAME)
	@# Check if container exists
	@if $(ENGINE) ps -a --format '{{.Names}}' | grep -q "^$(CONTAINER_NAME)$$"; then \
		echo "Container exists, starting..."; \
		$(ENGINE) start $(CONTAINER_NAME); \
	else \
		echo "Creating new container..."; \
		$(ENGINE) run -d \
			--name $(CONTAINER_NAME) \
			-e POSTGRES_DB=$(POSTGRES_DB) \
			-e POSTGRES_USER=$(POSTGRES_USER) \
			-e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) \
			-p $(POSTGRES_PORT):5432 \
			-v $(VOLUME_NAME):/var/lib/postgresql/data \
			--health-cmd="pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB)" \
			--health-interval=10s \
			--health-timeout=5s \
			--health-retries=5 \
			$(IMAGE_NAME):$(IMAGE_TAG); \
	fi
	@echo ""
	@echo "PostgreSQL is starting on localhost:$(POSTGRES_PORT)"
	@echo "  Database: $(POSTGRES_DB)"
	@echo "  User:     $(POSTGRES_USER)"
	@echo "  Password: $(POSTGRES_PASSWORD)"
	@echo ""
	@echo "Run 'make status' to check if it's ready"
	@echo "Run 'make psql' to connect"

## down: Stop PostgreSQL container
down: check-engine
	@echo "Stopping PostgreSQL..."
	@$(ENGINE) stop $(CONTAINER_NAME) 2>/dev/null || true
	@echo "PostgreSQL stopped"

## restart: Restart PostgreSQL container
restart: down up

## logs: View PostgreSQL logs
logs: check-engine
	$(ENGINE) logs -f $(CONTAINER_NAME)

## shell: Open shell in PostgreSQL container
shell: check-engine
	$(ENGINE) exec -it $(CONTAINER_NAME) sh

## psql: Connect to PostgreSQL with psql
psql: check-engine
	$(ENGINE) exec -it $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

## status: Check PostgreSQL status
status: check-engine
	@echo "Container Status:"
	@$(ENGINE) ps -a --filter "name=$(CONTAINER_NAME)" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
	@echo ""
	@echo "Health Check:"
	@$(ENGINE) exec $(CONTAINER_NAME) pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB) 2>/dev/null || echo "Container not running or not ready"

## clean: Remove PostgreSQL container and data
clean: check-engine
	@echo "WARNING: This will delete the container and ALL data!"
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	@$(ENGINE) stop $(CONTAINER_NAME) 2>/dev/null || true
	@$(ENGINE) rm $(CONTAINER_NAME) 2>/dev/null || true
	@$(ENGINE) volume rm $(VOLUME_NAME) 2>/dev/null || true
	@echo "PostgreSQL container and data removed"

## migrate: Run migrations manually (useful if schema changed)
migrate: check-engine
	@echo "Running migrations..."
	$(ENGINE) exec $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -f /docker-entrypoint-initdb.d/migrations/001_create_gpu_telemetry.sql
	$(ENGINE) exec $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -f /docker-entrypoint-initdb.d/migrations/002_create_gpus_view.sql
	@echo "Migrations complete"

## tables: List tables in the database
tables: check-engine
	@$(ENGINE) exec $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "\\dt"

## describe: Describe the gpu_telemetry table
describe: check-engine
	@$(ENGINE) exec $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "\\d gpu_telemetry"

## count: Count records in gpu_telemetry table
count: check-engine
	@$(ENGINE) exec $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "SELECT COUNT(*) as total_records FROM gpu_telemetry;"

## sample: Show sample records (10 rows)
sample: check-engine
	@$(ENGINE) exec $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "SELECT id, timestamp, metric_name, uuid, hostname, value FROM gpu_telemetry LIMIT 10;"

## gpus: List unique GPUs
gpus: check-engine
	@$(ENGINE) exec $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "SELECT DISTINCT uuid, model_name, hostname FROM gpu_telemetry;"

## rows: Show recent rows (50 at a time)
rows: check-engine
	@$(ENGINE) exec $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "SELECT id, timestamp, metric_name, gpu_index, uuid, hostname, value FROM gpu_telemetry ORDER BY timestamp DESC LIMIT 50;"

## all: Show ALL rows (use with caution for large datasets)
all: check-engine
	@$(ENGINE) exec $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "SELECT * FROM gpu_telemetry ORDER BY timestamp DESC;"

## tail: Show latest 20 records with all columns
tail: check-engine
	@$(ENGINE) exec $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "SELECT * FROM gpu_telemetry ORDER BY timestamp DESC LIMIT 20;"

## export: Export all data to CSV file
export: check-engine
	@$(ENGINE) exec $(CONTAINER_NAME) psql -U $(POSTGRES_USER) -d $(POSTGRES_DB) -c "COPY gpu_telemetry TO STDOUT WITH CSV HEADER;" > gpu_telemetry_export.csv
	@echo "Exported to gpu_telemetry_export.csv"

## wait: Wait for PostgreSQL to be ready (useful in scripts)
wait: check-engine
	@echo "Waiting for PostgreSQL to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if $(ENGINE) exec $(CONTAINER_NAME) pg_isready -U $(POSTGRES_USER) -d $(POSTGRES_DB) 2>/dev/null; then \
			echo "PostgreSQL is ready!"; \
			exit 0; \
		fi; \
		echo "Waiting... ($$i/10)"; \
		sleep 2; \
	done; \
	echo "PostgreSQL did not become ready in time"; \
	exit 1
