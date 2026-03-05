# ============================================================================
# GPU Telemetry Pipeline - Root Makefile
# ============================================================================
# One-command setup: make up
# One-command teardown: make down
# ============================================================================

# Configuration
DOCKER_REGISTRY ?= pp010
IMAGE_TAG ?= latest
KIND_CLUSTER_NAME ?= gpu-telemetry
NAMESPACE ?= gpu-telemetry
HELM_RELEASE ?= telemetry

# Detect container engine (podman or docker)
CONTAINER_ENGINE := $(shell which podman 2>/dev/null || which docker 2>/dev/null)
ifeq ($(CONTAINER_ENGINE),)
$(error No container engine found. Install docker or podman)
endif
ENGINE := $(notdir $(CONTAINER_ENGINE))

# ============================================================================
# QUICK START
# ============================================================================

.PHONY: up
up: check-deps cluster-create docker-build kind-load install-kyverno deploy wait status ## Full setup: cluster + build + deploy + Kyverno + port-forward
	@echo ""
	@echo "Starting port-forwards..."
	@kubectl port-forward svc/gateway 8080:8080 -n $(NAMESPACE) >/dev/null 2>&1 &
	@kubectl port-forward svc/mq 8082:8082 -n $(NAMESPACE) >/dev/null 2>&1 &
	@sleep 2
	@echo ""
	@echo "============================================"
	@echo "  Setup complete! APIs ready"
	@echo "============================================"
	@echo ""
	@echo "Gateway API:      http://localhost:8080"
	@echo "Gateway Swagger:  http://localhost:8080/swagger/index.html"
	@echo ""
	@echo "MQ Admin API:     http://localhost:8082"
	@echo "MQ Swagger:       http://localhost:8082/swagger/index.html"
	@echo ""
	@echo "Sample commands:"
	@echo "  curl http://localhost:8080/health"
	@echo "  curl http://localhost:8080/api/v1/gpus"
	@echo "  curl http://localhost:8082/api/v1/topics"
	@echo ""
	@echo "Scaling limits (enforced by Kyverno):"
	@echo "  • Streamer, Collector, Gateway: max 10 replicas"
	@echo "  • MQ, PostgreSQL: max 1 replica (cannot scale)"
	@echo ""
	@echo "Commands: make status | make logs | make down"
	@echo ""

.PHONY: down
down: ## Full teardown: undeploy + delete cluster + kill port-forwards
	@echo "Stopping port-forwards..."
	@pkill -f "kubectl port-forward.*gpu-telemetry" 2>/dev/null || true
	@$(MAKE) undeploy
	@$(MAKE) cluster-delete
	@echo ""
	@echo "============================================"
	@echo "  Teardown complete!"
	@echo "============================================"

.PHONY: demo
demo: ## Demo: show sample API commands
	@echo ""
	@echo "============================================"
	@echo "  GPU Telemetry Pipeline Demo"
	@echo "============================================"
	@echo ""
	@echo "Starting port-forward to Gateway..."
	@echo "Gateway: http://localhost:8080"
	@echo "Swagger: http://localhost:8080/swagger/index.html"
	@echo ""
	@echo "Sample commands (run in another terminal):"
	@echo "  curl http://localhost:8080/health"
	@echo "  curl http://localhost:8080/api/v1/gpus"
	@echo "  curl 'http://localhost:8080/api/v1/gpus/{uuid}/telemetry'"
	@echo ""
	@echo "Press Ctrl+C to stop port-forwarding"
	@kubectl port-forward svc/gateway 8080:8080 -n $(NAMESPACE)

# ============================================================================
# PREREQUISITES CHECK (auto-install on macOS if missing)
# ============================================================================

# Set AUTO_INSTALL=1 to automatically install missing dependencies
AUTO_INSTALL ?= 1

.PHONY: check-deps
check-deps: check-container check-kubectl check-helm check-kind ## Check all dependencies
	@echo ""
	@echo "All dependencies found!"

.PHONY: check-container
check-container:
	@if [ -z "$(CONTAINER_ENGINE)" ]; then \
		echo "❌ No container engine found. Install docker or podman"; \
		exit 1; \
	fi
	@echo "✓ Container engine: $(ENGINE)"

.PHONY: check-kubectl
check-kubectl:
	@if ! which kubectl > /dev/null 2>&1; then \
		if [ "$(AUTO_INSTALL)" = "1" ] && which brew > /dev/null 2>&1; then \
			echo "📦 Installing kubectl via Homebrew..."; \
			brew install kubectl; \
		else \
			echo "❌ kubectl not found. Install: brew install kubectl"; \
			exit 1; \
		fi; \
	fi
	@echo "✓ kubectl found"

.PHONY: check-helm
check-helm:
	@if ! which helm > /dev/null 2>&1; then \
		if [ "$(AUTO_INSTALL)" = "1" ] && which brew > /dev/null 2>&1; then \
			echo "📦 Installing helm via Homebrew..."; \
			brew install helm; \
		else \
			echo "❌ Helm not found. Install: brew install helm"; \
			exit 1; \
		fi; \
	fi
	@echo "✓ Helm found"

.PHONY: check-kind
check-kind:
	@if ! which kind > /dev/null 2>&1; then \
		if [ "$(AUTO_INSTALL)" = "1" ] && which brew > /dev/null 2>&1; then \
			echo "📦 Installing kind via Homebrew..."; \
			brew install kind; \
		else \
			echo "❌ Kind not found. Install: brew install kind"; \
			exit 1; \
		fi; \
	fi
	@echo "✓ Kind found"

# ============================================================================
# KIND CLUSTER
# ============================================================================

.PHONY: cluster-create
cluster-create: ## Create Kind cluster
	@if kind get clusters 2>/dev/null | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
		echo "Cluster '$(KIND_CLUSTER_NAME)' already exists"; \
	else \
		echo "Creating Kind cluster '$(KIND_CLUSTER_NAME)'..."; \
		kind create cluster --name $(KIND_CLUSTER_NAME) --config kind-config.yaml; \
	fi
	@kubectl cluster-info --context kind-$(KIND_CLUSTER_NAME)

.PHONY: cluster-delete
cluster-delete: ## Delete Kind cluster
	@echo "Deleting Kind cluster '$(KIND_CLUSTER_NAME)'..."
	@kind delete cluster --name $(KIND_CLUSTER_NAME) 2>/dev/null || true

.PHONY: cluster-status
cluster-status: ## Show cluster status
	@kubectl cluster-info --context kind-$(KIND_CLUSTER_NAME)
	@echo ""
	@kubectl get nodes

# ============================================================================
# BUILD
# ============================================================================

.PHONY: build-all
build-all: ## Build all Go binaries locally
	$(MAKE) -C mq build
	$(MAKE) -C streamer build
	$(MAKE) -C collector build
	$(MAKE) -C gateway build

.PHONY: docker-build
docker-build: ## Build all Docker images
	@echo "Building Docker images with $(ENGINE)..."
	$(ENGINE) build -t $(DOCKER_REGISTRY)/gpu-telemetry-mq:$(IMAGE_TAG) ./mq
	$(ENGINE) build -t $(DOCKER_REGISTRY)/gpu-telemetry-streamer:$(IMAGE_TAG) ./streamer
	$(ENGINE) build -t $(DOCKER_REGISTRY)/gpu-telemetry-collector:$(IMAGE_TAG) ./collector
	$(ENGINE) build -t $(DOCKER_REGISTRY)/gpu-telemetry-gateway:$(IMAGE_TAG) ./gateway
	$(ENGINE) build -t $(DOCKER_REGISTRY)/gpu-telemetry-db:$(IMAGE_TAG) ./db
	@echo "All images built successfully!"

.PHONY: docker-push
docker-push: ## Push images to DockerHub (requires: docker login)
	@echo "Pushing images to DockerHub..."
	$(ENGINE) push $(DOCKER_REGISTRY)/gpu-telemetry-mq:$(IMAGE_TAG)
	$(ENGINE) push $(DOCKER_REGISTRY)/gpu-telemetry-streamer:$(IMAGE_TAG)
	$(ENGINE) push $(DOCKER_REGISTRY)/gpu-telemetry-collector:$(IMAGE_TAG)
	$(ENGINE) push $(DOCKER_REGISTRY)/gpu-telemetry-gateway:$(IMAGE_TAG)
	$(ENGINE) push $(DOCKER_REGISTRY)/gpu-telemetry-db:$(IMAGE_TAG)
	@echo "All images pushed!"

.PHONY: kind-load
kind-load: ## Load images into Kind cluster (faster than registry)
	@echo "Loading images into Kind cluster..."
ifeq ($(ENGINE),podman)
	@echo "Using Podman - saving and loading via archive..."
	@mkdir -p /tmp/kind-images
	podman save -o /tmp/kind-images/mq.tar $(DOCKER_REGISTRY)/gpu-telemetry-mq:$(IMAGE_TAG)
	podman save -o /tmp/kind-images/streamer.tar $(DOCKER_REGISTRY)/gpu-telemetry-streamer:$(IMAGE_TAG)
	podman save -o /tmp/kind-images/collector.tar $(DOCKER_REGISTRY)/gpu-telemetry-collector:$(IMAGE_TAG)
	podman save -o /tmp/kind-images/gateway.tar $(DOCKER_REGISTRY)/gpu-telemetry-gateway:$(IMAGE_TAG)
	podman save -o /tmp/kind-images/db.tar $(DOCKER_REGISTRY)/gpu-telemetry-db:$(IMAGE_TAG)
	kind load image-archive /tmp/kind-images/mq.tar --name $(KIND_CLUSTER_NAME)
	kind load image-archive /tmp/kind-images/streamer.tar --name $(KIND_CLUSTER_NAME)
	kind load image-archive /tmp/kind-images/collector.tar --name $(KIND_CLUSTER_NAME)
	kind load image-archive /tmp/kind-images/gateway.tar --name $(KIND_CLUSTER_NAME)
	kind load image-archive /tmp/kind-images/db.tar --name $(KIND_CLUSTER_NAME)
	@rm -rf /tmp/kind-images
else
	kind load docker-image $(DOCKER_REGISTRY)/gpu-telemetry-mq:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(DOCKER_REGISTRY)/gpu-telemetry-streamer:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(DOCKER_REGISTRY)/gpu-telemetry-collector:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(DOCKER_REGISTRY)/gpu-telemetry-gateway:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
	kind load docker-image $(DOCKER_REGISTRY)/gpu-telemetry-db:$(IMAGE_TAG) --name $(KIND_CLUSTER_NAME)
endif
	@echo "All images loaded into Kind!"

# ============================================================================
# DEPLOY
# ============================================================================

.PHONY: deploy
deploy: ## Deploy to Kubernetes via Helm
	@echo "Deploying to Kubernetes..."
	@kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
ifeq ($(ENGINE),podman)
	@echo "Using Podman - images have localhost/ prefix in Kind"
	helm upgrade --install $(HELM_RELEASE) ./helm/gpu-telemetry \
		--namespace $(NAMESPACE) \
		--set global.image.registry=localhost/$(DOCKER_REGISTRY) \
		--set global.image.tag=$(IMAGE_TAG) \
		--set global.image.pullPolicy=Never \
		--wait --timeout 5m
else
	helm upgrade --install $(HELM_RELEASE) ./helm/gpu-telemetry \
		--namespace $(NAMESPACE) \
		--set global.image.registry=$(DOCKER_REGISTRY) \
		--set global.image.tag=$(IMAGE_TAG) \
		--set global.image.pullPolicy=IfNotPresent \
		--wait --timeout 5m
endif
	@echo "Deployment complete!"

.PHONY: undeploy
undeploy: ## Uninstall Helm release
	@echo "Uninstalling Helm release..."
	@helm uninstall $(HELM_RELEASE) --namespace $(NAMESPACE) 2>/dev/null || true
	@kubectl delete namespace $(NAMESPACE) 2>/dev/null || true
	@echo "Uninstall complete!"

.PHONY: wait
wait: ## Wait for all pods to be ready
	@echo "Waiting for pods to be ready..."
	@kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=$(HELM_RELEASE) -n $(NAMESPACE) --timeout=5m || true

.PHONY: status
status: ## Show deployment status
	@echo ""
	@echo "=== Namespace: $(NAMESPACE) ==="
	@echo ""
	@echo "--- Pods ---"
	@kubectl get pods -n $(NAMESPACE) -o wide 2>/dev/null || echo "No pods found"
	@echo ""
	@echo "--- Services ---"
	@kubectl get svc -n $(NAMESPACE) 2>/dev/null || echo "No services found"
	@echo ""
	@echo "--- Deployments ---"
	@kubectl get deployments -n $(NAMESPACE) 2>/dev/null || echo "No deployments found"

# ============================================================================
# SCALING
# ============================================================================

# Scaling limits
MAX_SCALABLE_REPLICAS := 10
MAX_SINGLETON_REPLICAS := 1

.PHONY: scale-streamer
scale-streamer: ## Scale streamer replicas (usage: make scale-streamer REPLICAS=3, max 10)
	@if [ $(REPLICAS) -gt $(MAX_SCALABLE_REPLICAS) ]; then \
		echo "❌ Error: Streamer max replicas is $(MAX_SCALABLE_REPLICAS), requested $(REPLICAS)"; \
		exit 1; \
	fi
	@if [ $(REPLICAS) -lt 1 ]; then \
		echo "❌ Error: Minimum replicas is 1"; \
		exit 1; \
	fi
	@kubectl scale deployment streamer --replicas=$(REPLICAS) -n $(NAMESPACE)
	@echo "✓ Streamer scaled to $(REPLICAS) replicas"

.PHONY: scale-collector
scale-collector: ## Scale collector replicas (usage: make scale-collector REPLICAS=3, max 10)
	@if [ $(REPLICAS) -gt $(MAX_SCALABLE_REPLICAS) ]; then \
		echo "❌ Error: Collector max replicas is $(MAX_SCALABLE_REPLICAS), requested $(REPLICAS)"; \
		exit 1; \
	fi
	@if [ $(REPLICAS) -lt 1 ]; then \
		echo "❌ Error: Minimum replicas is 1"; \
		exit 1; \
	fi
	@kubectl scale deployment collector --replicas=$(REPLICAS) -n $(NAMESPACE)
	@echo "✓ Collector scaled to $(REPLICAS) replicas"

.PHONY: scale-gateway
scale-gateway: ## Scale gateway replicas (usage: make scale-gateway REPLICAS=3, max 10)
	@if [ $(REPLICAS) -gt $(MAX_SCALABLE_REPLICAS) ]; then \
		echo "❌ Error: Gateway max replicas is $(MAX_SCALABLE_REPLICAS), requested $(REPLICAS)"; \
		exit 1; \
	fi
	@if [ $(REPLICAS) -lt 1 ]; then \
		echo "❌ Error: Minimum replicas is 1"; \
		exit 1; \
	fi
	@kubectl scale deployment gateway --replicas=$(REPLICAS) -n $(NAMESPACE)
	@echo "✓ Gateway scaled to $(REPLICAS) replicas"

# ============================================================================
# TESTING
# ============================================================================

.PHONY: test-unit
test-unit: ## Run unit tests for all services
	@echo "Running unit tests..."
	$(MAKE) -C mq test
	$(MAKE) -C streamer test
	$(MAKE) -C collector test
	$(MAKE) -C gateway test
	@echo "All unit tests passed!"

.PHONY: test-integration
test-integration: ## Run integration tests (requires test environment)
	@echo "Running integration tests..."
	$(MAKE) -C tests test

.PHONY: test-all
test-all: test-unit ## Run all tests
	@echo "All tests completed!"

.PHONY: coverage
coverage: ## Generate coverage report for all services
	@echo "Generating coverage reports..."
	$(MAKE) -C mq test-cover
	$(MAKE) -C streamer test-cover
	$(MAKE) -C collector test-cover
	$(MAKE) -C gateway test-cover
	@echo "Coverage reports generated in each service directory"

# ============================================================================
# DEVELOPMENT
# ============================================================================

.PHONY: port-forward
port-forward: ## Port-forward Gateway to localhost:8080
	@echo "Port-forwarding Gateway to http://localhost:8080"
	@echo "Swagger UI: http://localhost:8080/swagger/index.html"
	@echo "Press Ctrl+C to stop"
	@kubectl port-forward svc/gateway 8080:8080 -n $(NAMESPACE)

.PHONY: port-forward-mq
port-forward-mq: ## Port-forward MQ to localhost:8082 (HTTP) and 8081 (gRPC)
	@echo "Port-forwarding MQ..."
	@echo "HTTP: http://localhost:8082"
	@echo "gRPC: localhost:8081"
	@kubectl port-forward svc/mq 8081:8081 8082:8082 -n $(NAMESPACE)

.PHONY: logs
logs: ## Show logs from all pods (last 50 lines each)
	@echo "=== MQ Logs ==="
	@kubectl logs -l app=mq -n $(NAMESPACE) --tail=20 2>/dev/null || echo "No MQ logs"
	@echo ""
	@echo "=== Streamer Logs ==="
	@kubectl logs -l app=streamer -n $(NAMESPACE) --tail=20 2>/dev/null || echo "No Streamer logs"
	@echo ""
	@echo "=== Collector Logs ==="
	@kubectl logs -l app=collector -n $(NAMESPACE) --tail=20 2>/dev/null || echo "No Collector logs"
	@echo ""
	@echo "=== Gateway Logs ==="
	@kubectl logs -l app=gateway -n $(NAMESPACE) --tail=20 2>/dev/null || echo "No Gateway logs"

.PHONY: logs-follow
logs-follow: ## Follow logs from all pods
	@kubectl logs -f -l app.kubernetes.io/instance=$(HELM_RELEASE) -n $(NAMESPACE) --max-log-requests=10

.PHONY: swagger
swagger: ## Generate OpenAPI specs for all services
	$(MAKE) -C mq swagger
	$(MAKE) -C gateway swagger
	@echo "OpenAPI specs generated!"

# ============================================================================
# DATABASE INSPECTION
# ============================================================================

.PHONY: db-shell
db-shell: ## Open interactive PostgreSQL shell
	@echo "Connecting to PostgreSQL (password: postgres)..."
	@kubectl exec -it postgres-0 -n $(NAMESPACE) -- psql -U postgres -d telemetry

.PHONY: db-count
db-count: ## Show record counts in database
	@echo "=== Database Record Counts ==="
	@kubectl exec -it postgres-0 -n $(NAMESPACE) -- psql -U postgres -d telemetry -c \
		"SELECT 'gpu_telemetry' as table_name, COUNT(*) as records FROM gpu_telemetry UNION ALL SELECT 'gpus (view)', COUNT(*) FROM gpus;"

.PHONY: db-gpus
db-gpus: ## List all unique GPUs in database
	@echo "=== Unique GPUs ==="
	@kubectl exec -it postgres-0 -n $(NAMESPACE) -- psql -U postgres -d telemetry -c \
		"SELECT uuid, gpu_index, device, model_name, hostname, last_seen FROM gpus ORDER BY last_seen DESC LIMIT 20;"

.PHONY: db-telemetry
db-telemetry: ## Show recent telemetry records
	@echo "=== Recent Telemetry (last 10 records) ==="
	@kubectl exec -it postgres-0 -n $(NAMESPACE) -- psql -U postgres -d telemetry -c \
		"SELECT uuid, metric_name, value, timestamp FROM gpu_telemetry ORDER BY timestamp DESC LIMIT 10;"

.PHONY: db-stats
db-stats: ## Show database statistics
	@echo "=== Database Statistics ==="
	@kubectl exec -it postgres-0 -n $(NAMESPACE) -- psql -U postgres -d telemetry -c \
		"SELECT \
			(SELECT COUNT(*) FROM gpu_telemetry) as total_records, \
			(SELECT COUNT(DISTINCT uuid) FROM gpu_telemetry) as unique_gpus, \
			(SELECT COUNT(DISTINCT metric_name) FROM gpu_telemetry) as metric_types, \
			(SELECT MIN(timestamp) FROM gpu_telemetry) as oldest_record, \
			(SELECT MAX(timestamp) FROM gpu_telemetry) as newest_record;"

.PHONY: db-metrics
db-metrics: ## Show telemetry count by metric type
	@echo "=== Records by Metric Type ==="
	@kubectl exec -it postgres-0 -n $(NAMESPACE) -- psql -U postgres -d telemetry -c \
		"SELECT metric_name, COUNT(*) as count FROM gpu_telemetry GROUP BY metric_name ORDER BY count DESC LIMIT 15;"

.PHONY: db-all
db-all: ## Display all telemetry records in tabular format (usage: make db-all LIMIT=100)
	@echo "=== All Telemetry Records ==="
	@kubectl exec -it postgres-0 -n $(NAMESPACE) -- psql -U postgres -d telemetry -c \
		"SELECT id, uuid, gpu_index, device, model_name, hostname, metric_name, value, timestamp FROM gpu_telemetry ORDER BY timestamp DESC LIMIT $(or $(LIMIT),50);"

# ============================================================================
# HELM CHART PACKAGING & PUBLISHING
# ============================================================================

CHART_VERSION := 1.0.0
CHART_DIR := ./helm/gpu-telemetry
CHART_PACKAGE_DIR := ./dist

.PHONY: helm-package
helm-package: ## Package Helm chart for distribution
	@echo "Packaging Helm chart..."
	@mkdir -p $(CHART_PACKAGE_DIR)
	@helm package $(CHART_DIR) --destination $(CHART_PACKAGE_DIR) --version $(CHART_VERSION)
	@echo "Chart packaged: $(CHART_PACKAGE_DIR)/gpu-telemetry-$(CHART_VERSION).tgz"

.PHONY: helm-index
helm-index: helm-package ## Generate Helm repository index (for GitHub Pages)
	@echo "Generating Helm repository index..."
	@helm repo index $(CHART_PACKAGE_DIR) --url https://papanigr.github.io/gpu-telemetry-helm
	@echo "Index generated: $(CHART_PACKAGE_DIR)/index.yaml"

.PHONY: helm-push-oci
helm-push-oci: helm-package ## Push Helm chart to DockerHub as OCI artifact
	@echo "Pushing Helm chart to DockerHub OCI registry..."
	helm push $(CHART_PACKAGE_DIR)/gpu-telemetry-$(CHART_VERSION).tgz oci://registry-1.docker.io/$(DOCKER_REGISTRY)
	@echo ""
	@echo "Chart pushed! Install with:"
	@echo "  helm install telemetry oci://registry-1.docker.io/$(DOCKER_REGISTRY)/gpu-telemetry --version $(CHART_VERSION)"

.PHONY: publish-images
publish-images: docker-build docker-push ## Build and push all images to DockerHub
	@echo ""
	@echo "============================================"
	@echo "  Images published to DockerHub!"
	@echo "============================================"
	@echo ""
	@echo "Images available at:"
	@echo "  - docker.io/$(DOCKER_REGISTRY)/gpu-telemetry-mq:$(IMAGE_TAG)"
	@echo "  - docker.io/$(DOCKER_REGISTRY)/gpu-telemetry-streamer:$(IMAGE_TAG)"
	@echo "  - docker.io/$(DOCKER_REGISTRY)/gpu-telemetry-collector:$(IMAGE_TAG)"
	@echo "  - docker.io/$(DOCKER_REGISTRY)/gpu-telemetry-gateway:$(IMAGE_TAG)"
	@echo "  - docker.io/$(DOCKER_REGISTRY)/gpu-telemetry-db:$(IMAGE_TAG)"

.PHONY: publish-all
publish-all: publish-images helm-push-oci ## Publish images and Helm chart to DockerHub
	@echo ""
	@echo "============================================"
	@echo "  Published to DockerHub!"
	@echo "============================================"
	@echo ""
	@echo "Docker Images:"
	@echo "  - docker.io/$(DOCKER_REGISTRY)/gpu-telemetry-mq:$(IMAGE_TAG)"
	@echo "  - docker.io/$(DOCKER_REGISTRY)/gpu-telemetry-streamer:$(IMAGE_TAG)"
	@echo "  - docker.io/$(DOCKER_REGISTRY)/gpu-telemetry-collector:$(IMAGE_TAG)"
	@echo "  - docker.io/$(DOCKER_REGISTRY)/gpu-telemetry-gateway:$(IMAGE_TAG)"
	@echo "  - docker.io/$(DOCKER_REGISTRY)/gpu-telemetry-db:$(IMAGE_TAG)"
	@echo ""
	@echo "Helm Chart:"
	@echo "  - oci://registry-1.docker.io/$(DOCKER_REGISTRY)/gpu-telemetry:$(CHART_VERSION)"
	@echo ""
	@echo "To deploy (no source code needed!):"
	@echo ""
	@echo "  helm install telemetry oci://registry-1.docker.io/$(DOCKER_REGISTRY)/gpu-telemetry \\"
	@echo "    --version $(CHART_VERSION) \\"
	@echo "    --namespace gpu-telemetry --create-namespace"

# ============================================================================
# LOCAL DEVELOPMENT (without Kubernetes)
# ============================================================================

.PHONY: run-local
run-local: ## Run all services locally using containers
	@echo "Starting local development environment..."
	$(MAKE) -C db up
	$(MAKE) -C db wait
	@sleep 2
	$(MAKE) -C mq docker-run &
	@sleep 3
	$(MAKE) -C collector docker-run &
	$(MAKE) -C gateway docker-run &
	$(MAKE) -C streamer docker-run &
	@echo "All services starting..."

.PHONY: stop-local
stop-local: ## Stop local development environment
	@echo "Stopping local services..."
	-$(ENGINE) stop gpu-telemetry-mq gpu-telemetry-streamer gpu-telemetry-collector gpu-telemetry-gateway 2>/dev/null
	$(MAKE) -C db down

# ============================================================================
# CLEANUP
# ============================================================================

.PHONY: clean
clean: ## Clean build artifacts in all services
	$(MAKE) -C mq clean
	$(MAKE) -C streamer clean
	$(MAKE) -C collector clean
	$(MAKE) -C gateway clean
	@echo "Cleaned all build artifacts"

.PHONY: clean-images
clean-images: ## Remove all built Docker images
	-$(ENGINE) rmi $(DOCKER_REGISTRY)/gpu-telemetry-mq:$(IMAGE_TAG) 2>/dev/null
	-$(ENGINE) rmi $(DOCKER_REGISTRY)/gpu-telemetry-streamer:$(IMAGE_TAG) 2>/dev/null
	-$(ENGINE) rmi $(DOCKER_REGISTRY)/gpu-telemetry-collector:$(IMAGE_TAG) 2>/dev/null
	-$(ENGINE) rmi $(DOCKER_REGISTRY)/gpu-telemetry-gateway:$(IMAGE_TAG) 2>/dev/null
	-$(ENGINE) rmi $(DOCKER_REGISTRY)/gpu-telemetry-db:$(IMAGE_TAG) 2>/dev/null
	@echo "Removed all images"

.PHONY: clean-all
clean-all: clean clean-images cluster-delete ## Clean everything including cluster and images

# ============================================================================
# ADMISSION CONTROLLER (KYVERNO)
# ============================================================================

.PHONY: install-kyverno
install-kyverno: ## Install Kyverno admission controller
	@echo "Installing Kyverno..."
	@helm repo add kyverno https://kyverno.github.io/kyverno/ 2>/dev/null || true
	@helm repo update
	@helm upgrade --install kyverno kyverno/kyverno \
		--namespace kyverno \
		--create-namespace \
		--wait --timeout 3m
	@echo "✓ Kyverno installed"

.PHONY: enable-policies
enable-policies: ## Enable replica limit policies (requires Kyverno)
	@echo "Enabling admission policies..."
	@helm upgrade $(HELM_RELEASE) ./helm/gpu-telemetry \
		--namespace $(NAMESPACE) \
		--reuse-values \
		--set policies.enabled=true
	@echo "✓ Policies enabled"
	@echo ""
	@echo "Scaling limits now enforced by Kubernetes:"
	@echo "  - Streamer, Collector, Gateway: max 10 replicas"
	@echo "  - MQ, PostgreSQL: max 1 replica"

.PHONY: disable-policies
disable-policies: ## Disable replica limit policies
	@echo "Disabling admission policies..."
	@helm upgrade $(HELM_RELEASE) ./helm/gpu-telemetry \
		--namespace $(NAMESPACE) \
		--reuse-values \
		--set policies.enabled=false
	@echo "✓ Policies disabled"

.PHONY: test-policies
test-policies: ## Test that policies are enforced
	@echo "Testing scaling policies..."
	@echo ""
	@echo "Test 1: Try to scale MQ to 2 replicas (should fail)..."
	@kubectl scale deployment mq --replicas=2 -n $(NAMESPACE) 2>&1 || echo "✓ MQ scaling blocked as expected"
	@echo ""
	@echo "Test 2: Try to scale streamer to 15 replicas (should fail)..."
	@kubectl scale deployment streamer --replicas=15 -n $(NAMESPACE) 2>&1 || echo "✓ Streamer scaling blocked as expected"
	@echo ""
	@echo "Test 3: Scale streamer to 3 replicas (should succeed)..."
	@kubectl scale deployment streamer --replicas=3 -n $(NAMESPACE) && echo "✓ Streamer scaled to 3"
	@kubectl scale deployment streamer --replicas=1 -n $(NAMESPACE)

# ============================================================================
# HELP
# ============================================================================

.PHONY: help
help: ## Show this help
	@echo ""
	@echo "GPU Telemetry Pipeline"
	@echo "======================"
	@echo ""
	@echo "Quick Start:"
	@echo "  make up       - Create cluster, build images, deploy everything"
	@echo "  make demo     - Port-forward Gateway and show sample commands"
	@echo "  make down     - Teardown everything"
	@echo ""
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Configuration:"
	@echo "  DOCKER_REGISTRY=$(DOCKER_REGISTRY)"
	@echo "  IMAGE_TAG=$(IMAGE_TAG)"
	@echo "  KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME)"
	@echo "  NAMESPACE=$(NAMESPACE)"
	@echo "  AUTO_INSTALL=$(AUTO_INSTALL)  (set to 0 to disable auto-install of missing tools)"

.DEFAULT_GOAL := help
