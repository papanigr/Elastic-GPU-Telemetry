# GPU Telemetry Pipeline

An elastic, scalable telemetry pipeline for AI clusters with a custom message queue implementation.

## Quick Start

```bash
# One command to deploy everything (creates Kind cluster, builds images, deploys via Helm)
make up

# Demo the system (port-forwards Gateway, shows sample commands)
make demo

# Teardown everything
make down
```

---

## For Evaluators / Interviewers

This section provides a complete walkthrough to evaluate the project.

### Step 1: Prerequisites

Ensure these tools are installed:

```bash
# macOS (using Homebrew)
brew install docker    # or: brew install podman
brew install kubectl
brew install helm
brew install kind
brew install go        # Optional: for running tests locally
```

### Step 2: Clone and Deploy

```bash
# Clone the repository
git clone https://github.com/papanigr/Elastic-GPU-Telemetry.git
cd Elastic-GPU-Telemetry

# Deploy everything with one command
make up
```

**Expected output:**
```
✓ Container engine: docker
✓ kubectl found
✓ Helm found
✓ Kind found

Creating Kind cluster 'gpu-telemetry'...
Building Docker images...
Loading images into Kind...
Deploying to Kubernetes...
Installing Kyverno admission controller...
Enabling scaling policies...
Waiting for pods to be ready...

============================================
  Setup complete! Run 'make demo' to test
============================================

Scaling limits enforced by Kyverno policies:
  • Streamer, Collector, Gateway: max 10 replicas
  • MQ, PostgreSQL: max 1 replica (cannot scale)
```

### Step 3: Verify Deployment

```bash
# Check all pods are running
make status
```

**Expected output:**
```
--- Pods ---
NAME                         READY   STATUS    RESTARTS   AGE
collector-xxx                1/1     Running   0          1m
gateway-xxx                  1/1     Running   0          1m
mq-xxx                       1/1     Running   0          1m
postgres-0                   1/1     Running   0          1m
streamer-xxx                 1/1     Running   0          1m

--- Services ---
NAME       TYPE        CLUSTER-IP      PORT(S)
gateway    ClusterIP   10.96.x.x       8080/TCP
mq         ClusterIP   10.96.x.x       8081/TCP,8082/TCP
postgres   ClusterIP   None            5432/TCP
```

### Step 4: Test the API

```bash
# Start port-forwarding (runs in foreground)
make demo
```

**In a new terminal:**

```bash
# Health check
curl http://localhost:8080/health
# Expected: {"status":"healthy","database":"healthy","version":"1.0.0"}

# List all GPUs (wait ~30s for data to flow through)
curl http://localhost:8080/api/v1/gpus
# Expected: {"gpus":[{"uuid":"GPU-xxx",...}],"count":N}

# Get telemetry for a specific GPU
curl http://localhost:8080/api/v1/gpus/<GPU-UUID>/telemetry

# Open Swagger UI in browser
open http://localhost:8080/swagger/index.html
```

### Step 5: Test Scaling

**Scaling limits are enforced** by Kyverno admission controller (installed automatically):

| Component | Min | Max | Reason |
|-----------|-----|-----|--------|
| Streamer | 1 | 10 | Stateless, can parallelize |
| Collector | 1 | 10 | Stateless, MQ distributes load |
| Gateway | 1 | 10 | Stateless, reads from shared DB |
| MQ | 1 | 1 | In-memory state, cannot scale |
| PostgreSQL | 1 | 1 | Single-writer database |

#### Option 1: Makefile Commands (Recommended)

```bash
# Scale streamers to 3 replicas
make scale-streamer REPLICAS=3
# ✓ Streamer scaled to 3 replicas

# Scale collectors to 5 replicas
make scale-collector REPLICAS=5

# Scale gateway to 2 replicas
make scale-gateway REPLICAS=2

# Verify scaling
make status
```

#### Option 2: Helm Override (At deploy time)

```bash
# Deploy with custom replica counts
helm upgrade --install telemetry ./helm/gpu-telemetry \
  --namespace gpu-telemetry \
  --set streamer.replicas=3 \
  --set collector.replicas=5 \
  --set gateway.replicas=2
```

#### Option 3: Edit values.yaml (Change defaults)

Edit `helm/gpu-telemetry/values.yaml`:
```yaml
streamer:
  replicas: 3    # Default: 1, Max: 10

collector:
  replicas: 5    # Default: 1, Max: 10

gateway:
  replicas: 2    # Default: 1, Max: 10
```

Then redeploy: `make deploy`

#### Testing Policy Enforcement

```bash
# Try to exceed limit (blocked by Makefile)
make scale-streamer REPLICAS=15
# ❌ Error: Streamer max replicas is 10, requested 15

# Try to scale MQ directly (blocked by Kyverno)
kubectl scale deployment mq --replicas=2 -n gpu-telemetry
# Error: MQ must have exactly 1 replica (in-memory broker cannot scale horizontally)

# Try to scale PostgreSQL (blocked by Kyverno)
kubectl scale statefulset postgres --replicas=2 -n gpu-telemetry
# Error: PostgreSQL must have exactly 1 replica (single-writer database)
```

### Step 6: Run Tests

```bash
# Run all unit tests (with coverage)
make test-unit

# View coverage reports
make coverage
# Reports generated in each service: mq/coverage.html, gateway/coverage.html, etc.
```

### Step 7: View Logs

```bash
# View logs from all services
make logs

# Follow logs in real-time
make logs-follow
```

### Step 8: Cleanup

```bash
# Teardown everything (removes cluster and all resources)
make down
```

### Summary of Key Commands

| Command | What it does |
|---------|--------------|
| `make up` | Creates Kind cluster, builds images, deploys via Helm |
| `make demo` | Port-forwards Gateway, shows sample API commands |
| `make status` | Shows pods, services, deployments |
| `make scale-streamer REPLICAS=N` | Scale streamer pods (1-10) |
| `make scale-collector REPLICAS=N` | Scale collector pods (1-10) |
| `make scale-gateway REPLICAS=N` | Scale gateway pods (1-10) |
| `make test-unit` | Run all unit tests |
| `make coverage` | Generate coverage reports |
| `make logs` | View service logs |
| `make down` | Teardown everything |

### What to Evaluate

| Aspect | Where to Look |
|--------|---------------|
| **Custom MQ Implementation** | `mq/internal/broker/` - Topic-based pub/sub, consumer groups, DLQ |
| **Unit Tests** | Each service has `*_test.go` files, run `make test-unit` |
| **Integration Tests** | `tests/integration/` - 24 end-to-end tests |
| **OpenAPI Auto-generation** | `gateway/docs/`, `mq/docs/` - Generated from annotations |
| **Helm Charts** | `helm/gpu-telemetry/` - Kubernetes deployment |
| **Scalability** | Use `make scale-*` commands to test dynamic scaling |
| **Admission Controller** | Kyverno policies enforce replica limits (try scaling MQ to 2!) |
| **DLQ & Resilience** | See `mq/README.md` for DLQ architecture |
| **AI Usage Documentation** | `docs/AI-USAGE.md` - Prompts, outcomes, manual interventions |
| **Design Decisions** | `docs/DESIGN.md` - Detailed architecture document |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           GPU TELEMETRY PIPELINE                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────┐    ┌─────────────────────┐    ┌─────────────────────────┐ │
│  │  CSV File   │    │   CUSTOM MESSAGE    │    │                         │ │
│  │  (Source)   │    │       QUEUE         │    │      STORAGE            │ │
│  └──────┬──────┘    │                     │    │    (PostgreSQL)         │ │
│         │           │  ┌───────────────┐  │    │                         │ │
│         ▼           │  │    Broker     │  │    └────────────▲────────────┘ │
│  ┌──────────────┐   │  │   Service     │  │                 │              │
│  │  Telemetry   │   │  │               │  │    ┌────────────┴────────────┐ │
│  │  Streamer    ├───┼─►│  - Topics     │  │    │                         │ │
│  │  (1-10)      │   │  │  - DLQ        │──┼───►│   Telemetry Collector   │ │
│  └──────────────┘   │  │  - Pub/Sub    │  │    │       (1-10)            │ │
│                     │  └───────────────┘  │    │                         │ │
│                     │                     │    └────────────┬────────────┘ │
│                     └─────────────────────┘                 │              │
│                                                             │              │
│                     ┌─────────────────────┐                 │              │
│                     │    API Gateway      │◄────────────────┘              │
│                     │                     │                                │
│                     │  GET /api/v1/gpus   │                                │
│                     │  GET /api/v1/gpus/  │                                │
│                     │      {id}/telemetry │                                │
│                     └──────────┬──────────┘                                │
│                                │                                           │
└────────────────────────────────┼───────────────────────────────────────────┘
                                 │
                                 ▼
                          ┌──────────────┐
                          │   Clients    │
                          └──────────────┘
```

## Components

| Component | Description | Scaling | Why |
|-----------|-------------|---------|-----|
| **Streamer** | Reads CSV telemetry, streams to MQ via gRPC | 1-10 replicas | Stateless, parallelizable |
| **MQ Broker** | Custom message queue with DLQ, topics, consumer groups | 1 replica only | In-memory state (see [Scaling Constraints](#scaling-constraints)) |
| **Collector** | Consumes from MQ, persists to PostgreSQL | 1-10 replicas | Stateless, MQ distributes load |
| **Gateway** | REST API for querying telemetry | 1-10 replicas | Stateless, reads from shared DB |
| **PostgreSQL** | Telemetry storage | 1 replica only | Single-writer DB (see [Scaling Constraints](#scaling-constraints)) |

## Prerequisites

| Tool | Install Command | Purpose |
|------|-----------------|---------|
| Docker/Podman | `brew install docker` or `brew install podman` | Container runtime |
| kubectl | `brew install kubectl` | Kubernetes CLI |
| Helm | `brew install helm` | Kubernetes package manager |
| Kind | `brew install kind` | Local Kubernetes cluster |
| Go 1.21+ | `brew install go` | For local development |

## Installation

### Option 1: One-Command Setup (Recommended)

```bash
# Clone the repository
git clone https://github.com/papanigr/Elastic-GPU-Telemetry.git
cd Elastic-GPU-Telemetry

# Deploy everything
make up
```

This will:
1. Check all prerequisites
2. Create a Kind cluster
3. Build all Docker images
4. Load images into Kind
5. Deploy via Helm
6. Wait for pods to be ready

### Option 2: Step-by-Step

```bash
# 1. Check dependencies
make check-deps

# 2. Create Kind cluster
make cluster-create

# 3. Build Docker images
make docker-build

# 4. Load images into Kind
make kind-load

# 5. Deploy via Helm
make deploy

# 6. Check status
make status
```

### Option 3: Use Pre-built DockerHub Images

```bash
# Create cluster and deploy using pre-built images from DockerHub
make cluster-create
make deploy DOCKER_REGISTRY=pp010 IMAGE_TAG=latest
```

### Option 4: Deploy Without Source Code (Production)

If images are published to DockerHub, you only need the Helm chart:

```bash
# 1. Create a Kubernetes cluster (Kind, EKS, GKE, etc.)
kind create cluster --name gpu-telemetry

# 2. Install directly from DockerHub OCI registry (no download needed!)
helm install telemetry oci://registry-1.docker.io/pp010/gpu-telemetry \
  --version 1.0.0 \
  --namespace gpu-telemetry \
  --create-namespace

# OR download the chart first:
# Option A: From GitHub release
wget https://github.com/papanigr/AI-Infra-Assignment/releases/download/v1.0.0/gpu-telemetry-1.0.0.tgz
helm install telemetry gpu-telemetry-1.0.0.tgz --namespace gpu-telemetry --create-namespace

# Option B: From Helm repository (GitHub Pages)
helm repo add gpu-telemetry https://papanigr.github.io/gpu-telemetry-helm
helm repo update
helm install telemetry gpu-telemetry/gpu-telemetry --namespace gpu-telemetry --create-namespace

# 4. Verify deployment
kubectl get pods -n gpu-telemetry

# 5. Access the API
kubectl port-forward svc/gateway 8080:8080 -n gpu-telemetry
curl http://localhost:8080/health
```

**What you need to distribute:**
| Asset | Required | Purpose |
|-------|----------|---------|
| Source code | ❌ No | Not needed for deployment |
| Docker images | ✅ Yes | On DockerHub (public or private) |
| Helm chart | ✅ Yes | `.tgz` file or Helm repository |

**For maintainers - publish to DockerHub:**
```bash
# 1. Login to DockerHub (required once)
podman login docker.io
# or: docker login docker.io

# 2. Publish everything (images + Helm chart)
make publish-all

# This pushes:
# - 5 Docker images → docker.io/pp010/gpu-telemetry-*:latest
# - 1 Helm chart   → oci://registry-1.docker.io/pp010/gpu-telemetry:1.0.0
```

**Individual publishing commands:**
```bash
make docker-push       # Push Docker images only
make helm-push-oci     # Push Helm chart to OCI registry only
make helm-package      # Package chart locally (dist/gpu-telemetry-1.0.0.tgz)
```

## Usage

### Access the API

```bash
# Port-forward the Gateway
make demo
# or
kubectl port-forward svc/gateway 8080:8080 -n gpu-telemetry
```

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/api/v1/gpus` | List all GPUs |
| GET | `/api/v1/gpus/{id}/telemetry` | Get telemetry for a GPU |
| GET | `/api/v1/gpus/{id}/telemetry?start_time=...&end_time=...` | Filtered telemetry |
| GET | `/swagger/index.html` | Swagger UI |

### Example Requests

```bash
# Health check
curl http://localhost:8080/health

# List all GPUs
curl http://localhost:8080/api/v1/gpus

# Get telemetry for a specific GPU
curl http://localhost:8080/api/v1/gpus/GPU-abc123/telemetry

# Get telemetry with time filter
curl "http://localhost:8080/api/v1/gpus/GPU-abc123/telemetry?start_time=2026-01-28&end_time=2026-01-29"
```

## Scaling

```bash
# Scale streamers (1-10)
make scale-streamer REPLICAS=5

# Scale collectors (1-10)
make scale-collector REPLICAS=5

# Scale gateway (1-10)
make scale-gateway REPLICAS=3

# Check status
make status
```

### Scaling Constraints

| Component | Max Replicas | Can Scale? | Why |
|-----------|-------------|------------|-----|
| **Streamer** | 10 | ✅ Yes | Stateless - each instance reads CSV and publishes to MQ |
| **Collector** | 10 | ✅ Yes | Stateless - MQ distributes messages across consumer group |
| **Gateway** | 10 | ✅ Yes | Stateless - all instances read from same PostgreSQL |
| **MQ Broker** | 1 | ❌ No | In-memory state - replicas would have separate queues |
| **PostgreSQL** | 1 | ❌ No | Single-writer database - requires replication setup |

#### Why MQ Cannot Scale Horizontally

The custom MQ is an **in-memory message broker**. If you run multiple replicas:

```
┌─────────────────────────────────────────────────────────┐
│              Multiple MQ Replicas = Broken              │
├─────────────────────────────────────────────────────────┤
│                                                         │
│   Streamer ──publish──▶ MQ Pod-1 (has message)         │
│                                                         │
│   Collector ◀──subscribe── MQ Pod-2 (empty!)           │
│                                                         │
│   Result: Message never delivered!                      │
└─────────────────────────────────────────────────────────┘
```

**To scale MQ horizontally, you would need:**
- Shared state backend (Redis, etcd)
- Or use a production MQ (Kafka, NATS, RabbitMQ)

For this demo, single MQ instance handles the load. The bottleneck is typically database writes, not the broker.

#### Why PostgreSQL Cannot Scale Horizontally

PostgreSQL is a **single-writer RDBMS**. Simply adding replicas creates data inconsistency:

```
┌─────────────────────────────────────────────────────────┐
│           Multiple PostgreSQL = Data Corruption         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│   Pod-1: INSERT INTO telemetry VALUES (...)  ──┐       │
│   Pod-2: INSERT INTO telemetry VALUES (...)  ──┼─ ???  │
│   Pod-3: INSERT INTO telemetry VALUES (...)  ──┘       │
│                                                         │
│   Each pod has its own PVC. No shared state.            │
│   Writes to Pod-1 are invisible to Pod-2!               │
└─────────────────────────────────────────────────────────┘
```

**Production scaling options:**

| Approach | Description | Use Case |
|----------|-------------|----------|
| **Read Replicas** | 1 primary (writes) + N replicas (reads) | Read-heavy workloads |
| **Patroni/HA** | Automatic failover, still 1 writer | High availability |
| **Citus** | Distributed PostgreSQL with sharding | Horizontal writes |
| **Managed DB** | AWS RDS, GCP Cloud SQL | Production deployments |

For this demo, a single PostgreSQL instance is appropriate and sufficient.

### Enforcing Scaling Limits

Scaling limits are enforced at two levels:

**1. Makefile Validation (Default)**
```bash
# These commands validate limits before scaling
make scale-streamer REPLICAS=15
# ❌ Error: Streamer max replicas is 10, requested 15

make scale-streamer REPLICAS=5
# ✓ Streamer scaled to 5 replicas
```

**2. Kubernetes Admission Controller (Optional - Kyverno)**

For production-grade enforcement at the Kubernetes API level:

```bash
# Install Kyverno admission controller
make install-kyverno

# Enable replica limit policies
make enable-policies

# Test policies are enforced
make test-policies
```

With policies enabled, even direct `kubectl` commands are blocked:

```bash
kubectl scale deployment mq --replicas=2 -n gpu-telemetry
# Error: MQ must have exactly 1 replica (in-memory broker cannot scale horizontally)

kubectl scale deployment streamer --replicas=15 -n gpu-telemetry
# Error: Streamer replicas must be between 1 and 10
```

| Component | Limit | Enforced By |
|-----------|-------|-------------|
| Streamer | 1-10 | Makefile + Kyverno |
| Collector | 1-10 | Makefile + Kyverno |
| Gateway | 1-10 | Makefile + Kyverno |
| MQ | 1 only | Makefile + Kyverno |
| PostgreSQL | 1 only | Kyverno (StatefulSet) |

## Testing

```bash
# Run all unit tests
make test-unit

# Run integration tests (requires test environment)
make test-integration

# Generate coverage reports
make coverage
```

## Project Structure

```
gpu-telemetry-pipeline/
├── Makefile                 # Root orchestration
├── kind-config.yaml         # Kind cluster config
├── helm/
│   └── gpu-telemetry/       # Helm chart
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
├── mq/                      # Custom Message Queue
│   ├── cmd/main.go
│   ├── internal/
│   │   ├── api/             # HTTP handlers
│   │   ├── broker/          # Core broker logic
│   │   ├── config/
│   │   └── grpc/            # gRPC server
│   ├── Dockerfile
│   └── README.md
├── streamer/                # Telemetry Streamer
│   ├── cmd/main.go
│   ├── internal/
│   ├── data/dcgm_metrics.csv
│   ├── Dockerfile
│   └── README.md
├── collector/               # Telemetry Collector
│   ├── cmd/main.go
│   ├── internal/
│   ├── Dockerfile
│   └── README.md
├── gateway/                 # API Gateway
│   ├── cmd/main.go
│   ├── internal/
│   ├── docs/                # Auto-generated OpenAPI
│   ├── Dockerfile
│   └── README.md
├── db/                      # PostgreSQL
│   ├── migrations/
│   ├── Dockerfile
│   └── README.md
├── tests/                   # Integration tests
│   ├── integration/
│   └── README.md
└── docs/
    └── DESIGN.md            # Detailed design document
```

## Configuration

### Environment Variables

See individual service READMEs for full configuration:
- [MQ Configuration](mq/README.md#configuration)
- [Streamer Configuration](streamer/README.md#configuration)
- [Collector Configuration](collector/README.md#configuration)
- [Gateway Configuration](gateway/README.md#configuration)

### Helm Values

Override defaults in `helm/gpu-telemetry/values.yaml`:

```bash
# Deploy with custom values
helm upgrade --install telemetry ./helm/gpu-telemetry \
  --set streamer.replicas=3 \
  --set collector.replicas=3 \
  --namespace gpu-telemetry
```

### Deploy Individual Components

The umbrella chart allows enabling/disabling components:

```bash
# Deploy only MQ and PostgreSQL (infrastructure only)
helm upgrade --install telemetry ./helm/gpu-telemetry \
  --namespace gpu-telemetry \
  --set streamer.enabled=false \
  --set collector.enabled=false \
  --set gateway.enabled=false

# Deploy without PostgreSQL (use external database)
helm upgrade --install telemetry ./helm/gpu-telemetry \
  --namespace gpu-telemetry \
  --set postgres.enabled=false \
  --set global.postgres.host=my-external-db.example.com

# Deploy only Gateway (for read-only API access)
helm upgrade --install telemetry ./helm/gpu-telemetry \
  --namespace gpu-telemetry \
  --set mq.enabled=false \
  --set streamer.enabled=false \
  --set collector.enabled=false
```

### Component Enable/Disable Flags

| Component | Flag | Default |
|-----------|------|---------|
| MQ Broker | `mq.enabled` | `true` |
| Streamer | `streamer.enabled` | `true` |
| Collector | `collector.enabled` | `true` |
| Gateway | `gateway.enabled` | `true` |
| PostgreSQL | `postgres.enabled` | `true` |
| Kyverno Policies | `policies.enabled` | `false` (enabled via `make up`) |

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make up` | Full setup: cluster + build + deploy |
| `make down` | Full teardown |
| `make demo` | Port-forward and show sample commands |
| `make status` | Show deployment status |
| `make logs` | Show logs from all services |
| `make test-unit` | Run unit tests |
| `make coverage` | Generate coverage reports |
| `make swagger` | Generate OpenAPI specs |
| `make clean` | Clean build artifacts |
| `make help` | Show all available targets |
| **Scaling** | |
| `make scale-streamer REPLICAS=N` | Scale streamer (1-10, enforced) |
| `make scale-collector REPLICAS=N` | Scale collector (1-10, enforced) |
| `make scale-gateway REPLICAS=N` | Scale gateway (1-10, enforced) |
| **Admission Controller** | |
| `make install-kyverno` | Install Kyverno admission controller |
| `make enable-policies` | Enable Kubernetes-level scaling policies |
| `make test-policies` | Test that policies block invalid scaling |
| **Publishing** | |
| `make docker-push` | Push images to DockerHub |
| `make publish-images` | Build and push all images |
| `make helm-package` | Package Helm chart to `dist/` |
| `make helm-push-oci` | Push Helm chart to DockerHub OCI registry |
| `make publish-all` | Publish images + Helm chart to DockerHub |

## Key Features

### Custom Message Queue

- **Topic-based pub/sub** with consumer groups
- **At-least-once delivery** with acknowledgments
- **Dead Letter Queue (DLQ)** with auto-replay
- **Dual protocol**: gRPC (inter-service) + HTTP (admin)
- See [MQ README](mq/README.md) for details

### Scalability

- Streamers and Collectors scale independently (1-10 replicas)
- Consumer groups provide load balancing
- MQ handles backpressure with configurable queue limits

### Resilience

- DLQ for failed message handling
- Auto-replay of failed messages
- Graceful shutdown handling
- Health checks on all services

## OpenAPI Documentation

OpenAPI specs are **auto-generated** from code annotations:

```bash
# Generate specs
make swagger

# Access Swagger UI (after deploying)
open http://localhost:8080/swagger/index.html  # Gateway
open http://localhost:8082/swagger/index.html  # MQ (via port-forward)
```

## Troubleshooting

### Pods not starting

```bash
# Check pod status
kubectl get pods -n gpu-telemetry

# Check pod logs
kubectl logs -l app=mq -n gpu-telemetry

# Describe pod for events
kubectl describe pod <pod-name> -n gpu-telemetry
```

### Database connection issues

```bash
# Check PostgreSQL is running
kubectl get pods -l app=postgres -n gpu-telemetry

# Check PostgreSQL logs
kubectl logs -l app=postgres -n gpu-telemetry
```

### Kind cluster issues

```bash
# Delete and recreate cluster
make cluster-delete
make cluster-create
```

## Development

### Run Locally (without Kubernetes)

```bash
# Start all services locally using containers
make run-local

# Stop local services
make stop-local
```

### Build Individual Services

```bash
# Build a specific service
cd mq && make build
cd streamer && make build
cd collector && make build
cd gateway && make build
```

## Documentation

- [Design Document](docs/DESIGN.md) - Detailed architecture and design decisions
- [AI Usage](docs/AI-USAGE.md) - How AI assistance was used in development
- [MQ Service](mq/README.md) - Custom message queue documentation
- [Streamer Service](streamer/README.md) - Telemetry streamer documentation
- [Collector Service](collector/README.md) - Telemetry collector documentation
- [Gateway Service](gateway/README.md) - API gateway documentation
- [Database](db/README.md) - PostgreSQL setup and schema
- [Integration Tests](tests/README.md) - System test documentation

## License

MIT
