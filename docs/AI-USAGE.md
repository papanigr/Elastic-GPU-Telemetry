# AI Usage Documentation

This document details how AI assistance (Claude/Cursor) was used throughout the development of the GPU Telemetry Pipeline project, including prompts, outcomes, and areas requiring manual intervention.

## Table of Contents
1. [Development Approach](#development-approach)
2. [Project Bootstrapping](#project-bootstrapping)
3. [Code Development](#code-development)
4. [Unit Test Development](#unit-test-development)
5. [Build Environment & Deployment](#build-environment--deployment)
6. [Documentation](#documentation)
7. [Where AI Fell Short](#where-ai-fell-short)
8. [Summary Statistics](#summary-statistics)

---

## Development Approach

The project was developed using an **iterative AI-assisted workflow**:

1. **Prompt** → Describe the requirement or problem
2. **AI Response** → Receive code/design suggestions
3. **Review** → Evaluate the output
4. **Iterate** → Refine with follow-up prompts or manual fixes
5. **Verify** → Test and validate the implementation

**AI Tool Used**: Claude Opus 4.5 High-Thinking (claude-opus-4-5-20250414) via Cursor IDE Agent Mode

---

## Project Bootstrapping

### Initial Project Structure

**Prompt**:
> "Read the PDF and suggest me the progress. Create a plan for implementing the GPU Telemetry Pipeline with a custom message queue."

**AI Contribution**:
- Analyzed PDF requirements
- Proposed microservices architecture (Streamer, MQ, Collector, Gateway, DB)
- Suggested project structure with separate Go modules per service
- Recommended technology choices (chi router, zerolog, sqlx)

**Outcome**: AI created the initial project layout with:
```
├── mq/           # Custom Message Queue
├── streamer/     # Telemetry Streamer
├── collector/    # Telemetry Collector
├── gateway/      # API Gateway
├── db/           # PostgreSQL
├── helm/         # Kubernetes deployment
└── tests/        # Integration tests
```

**Manual Intervention**: None - structure was accepted as proposed.

---

### Go Module Initialization

**Prompt**:
> "Initialize the Go modules for each service with proper dependencies."

**AI Contribution**:
- Created `go.mod` files for each service
- Added required dependencies (chi, zerolog, sqlx, grpc)
- Set up proper module paths

**Manual Intervention**: 
- Had to update Go version compatibility (1.21 → 1.23) due to local toolchain
- Fixed protobuf plugin versions for compatibility

---

## Code Development

### Custom Message Queue (MQ)

**Prompt**:
> "Implement a custom message queue with topic-based pub/sub, consumer groups, and at-least-once delivery. Use gRPC for inter-service communication."

**AI Contribution**:
- Designed broker architecture with topics and consumer groups
- Implemented gRPC service for Publish/Subscribe/Ack operations
- Added HTTP admin APIs for monitoring
- Created message lifecycle management (Pending → Delivered → Acked)

**Follow-up Prompt**:
> "Add a Dead Letter Queue (DLQ) with automatic replay for failed messages."

**AI Contribution**:
- Designed DLQ with retry counting
- Implemented auto-replay mechanism with configurable delays
- Added DLQ admin APIs (list, replay, delete)
- Created "Dead" state for permanently failed messages

**Manual Intervention**:
- Fixed a critical bug where `MQ_MAX_MESSAGE_AGE` was dropping messages before they could enter DLQ
- The fix required understanding the cleanup logic and adding conditions to preserve messages being retried

**Bug Fix Prompt**:
> "DLQ is not engaging when the database is down. Messages are being dropped."

**AI Analysis**: Identified that messages with `RetryCount > 0` were being cleaned up by age before entering DLQ.

**Fix Applied**:
```go
// Before: Messages dropped if older than maxAge
if !msg.IsDLQ && now.Sub(msg.Timestamp) > maxAge {
    result.Removed++
    continue
}

// After: Only drop if Pending with no retries
if !msg.IsDLQ && now.Sub(msg.Timestamp) > maxAge {
    if msg.State == models.Pending && msg.RetryCount == 0 {
        result.Removed++
        continue
    }
}
```

---

### Telemetry Streamer

**Prompt**:
> "Create a streamer that reads GPU telemetry from CSV and publishes to the MQ via gRPC."

**AI Contribution**:
- Implemented CSV reader with looping capability
- Created gRPC client for publishing to MQ
- Added batch publishing with configurable intervals
- Implemented graceful shutdown

**Manual Intervention**:
- CSV file path mounting in Docker required adjustment
- Added CSV file to Docker image (was initially expecting external mount)

---

### Telemetry Collector

**Prompt**:
> "Create a collector that consumes from MQ, parses telemetry, and persists to PostgreSQL with batch writes."

**AI Contribution**:
- Implemented gRPC consumer with acknowledgments
- Created worker pool for parallel processing
- Added batch database inserts for efficiency
- Implemented NACK on failure (triggers DLQ flow)

**Manual Intervention**: None - worked as expected.

---

### API Gateway

**Prompt**:
> "Create a REST API Gateway with endpoints: GET /api/v1/gpus and GET /api/v1/gpus/{id}/telemetry with time filters. Auto-generate OpenAPI spec."

**AI Contribution**:
- Implemented handlers with chi router
- Added Swagger annotations for auto-generation
- Created repository layer for PostgreSQL queries
- Added health checks and Swagger UI

**Follow-up Issue**:
> "/openapi.yaml returns 404"

**AI Fix**: Changed route from `/openapi.yaml` to use swaggo's built-in Swagger UI at `/swagger/index.html`.

---

### PostgreSQL Schema

**Prompt**:
> "Design a PostgreSQL schema for GPU telemetry with proper indexes for time-range queries."

**AI Contribution**:
- Created `gpu_telemetry` table with all DCGM fields
- Added composite indexes for GPU UUID + timestamp queries
- Created `gpus` view for listing unique GPUs
- Added migration scripts

**Manual Intervention**: None.

---

## Unit Test Development

### Test Generation

**Prompt**:
> "Run unit tests and report coverage for collector, mq, and streamer services, ignoring generated code."

**AI Contribution**:
- Created test files for all services (`*_test.go`)
- Used table-driven tests (idiomatic Go)
- Mocked dependencies (database, MQ client)
- Achieved coverage targets

**Coverage Results**:
| Service | Coverage |
|---------|----------|
| MQ Broker | 75%+ |
| Collector | 70%+ |
| Streamer | 65%+ |
| Gateway | 70%+ |

**Prompt for Test Improvements**:
> "Add tests for DLQ functionality including retry limits and dead state."

**AI Contribution**:
- Added DLQ-specific test cases
- Tested retry counting and state transitions
- Verified auto-replay timing

**Manual Intervention**:
- Some test timing issues required adjustment (race conditions in async tests)
- Increased timeouts for CI environment stability

---

### Integration Tests

**Prompt**:
> "Create system/integration tests in a separate tests/ folder without modifying main service code."

**AI Contribution**:
- Created `tests/integration/` module with Go build tags
- Implemented 24 end-to-end tests
- Added test environment setup with containers
- Created Makefile targets for running tests

**Issue Encountered**:
> "Integration tests fail with 'connection refused' to localhost:8085"

**AI Analysis**: `localhost` on macOS resolves to IPv6 (`::1`) while containers bind to IPv4 (`0.0.0.0`).

**Fix Applied**: Changed test config to use `127.0.0.1` explicitly.

---

## Build Environment & Deployment

### Dockerfile Creation

**Prompt**:
> "Create multi-stage Dockerfiles for each service with minimal final images."

**AI Contribution**:
- Created multi-stage builds (builder → runtime)
- Used Alpine base images for small size
- Added proper user permissions (non-root)
- Included health checks

**Issue Encountered**:
> "Streamer crashes with 'no such file or directory' for CSV file"

**AI Fix**: Modified Dockerfile to copy CSV data file into the image instead of requiring external mount.

---

### Helm Charts

**Prompt**:
> "Create Helm charts for Kubernetes deployment. Use a single umbrella chart, Kind cluster, and DockerHub for images."

**AI Contribution**:
- Created `helm/gpu-telemetry/` umbrella chart
- Added templates for all services (Deployments, Services, StatefulSet)
- Created ConfigMaps and Secrets
- Added init containers for startup ordering

**Prompt**:
> "Create a root Makefile that handles end-to-end deployment for a first-time user."

**AI Contribution**:
- Created comprehensive Makefile with:
  - `make up` - One-command full setup (includes port-forward)
  - `make down` - Full teardown (kills port-forwards, deletes cluster)
  - `make status` - Show deployment status
  - `make logs` - View logs from all services
  - `make scale-*` - Scale services with validation
  - Prerequisite checks with auto-install (Homebrew on macOS)

**Follow-up Prompt**:
> "Why can't make demo and make up be done in same place, it's easy right?"

**AI Improvement**:
- Updated `make up` to automatically start port-forward in background after deployment
- Updated `make down` to kill port-forward processes before teardown
- Removed separate `make demo` command (no longer needed)
- Result: Single command (`make up`) now provides fully accessible API at `localhost:8080`

```bash
# One command does everything
make up    # Deploy + port-forward, API ready immediately at localhost:8080
make down  # Kill port-forward + full cleanup
```

**Follow-up Prompt**:
> "And make down to clear everything?"

**AI Improvement**:
- Added `pkill` command to kill background port-forward processes
- Ensures clean teardown with no orphan processes

---

### Podman/Kind Compatibility

**Issue Encountered**:
> "kind load docker-image fails with 'image not present locally' when using Podman"

**AI Analysis**: Podman stores images with `localhost/` prefix which Kind doesn't recognize.

**Solution Applied**:
```makefile
# Use podman save + kind load image-archive
podman save -o /tmp/kind-images/mq.tar papanigr/gpu-telemetry-mq:latest
kind load image-archive /tmp/kind-images/mq.tar --name gpu-telemetry
```

**Follow-up Issue**:
> "Pods in ImagePullBackOff after loading images"

**AI Fix**: Updated Helm deployment to set `imagePullPolicy: Never` and correct registry prefix for Podman.

---

### Admission Controller (Kyverno)

**Prompt**:
> "Add admission controllers to restrict scaling. Max 10 replicas for streamer, collector, gateway. Max 1 for MQ and PostgreSQL."

**AI Contribution**:
- Created Kyverno ClusterPolicies for replica limits
- Added Makefile validation for scale commands
- Integrated Kyverno installation into `make up`
- Created `make test-policies` for verification

**Manual Intervention**: None - worked as designed.

---

### DockerHub Publishing

**Prompt**:
> "Host all images in my DockerHub and provide only Helm command to deploy everything, without sharing source code."

**Follow-up**:
> "This is my DockerHub account: https://hub.docker.com/u/pp010"
> "This is my GitHub account: https://github.com/papanigr/"

**AI Contribution**:
- Identified user accounts: DockerHub (`pp010`), GitHub (`papanigr`)
- Added `make publish-images` to push all 5 Docker images to DockerHub
- Added `make helm-push-oci` to push Helm chart to DockerHub OCI registry
- Added `make publish-all` to publish everything in one command
- Updated `DOCKER_REGISTRY` in Makefile and `values.yaml` to `pp010`
- Updated README with deployment instructions for users without source code

**Published Assets**:
| Asset | Location |
|-------|----------|
| Docker Images | `docker.io/pp010/gpu-telemetry-*:latest` |
| Helm Chart | `oci://registry-1.docker.io/pp010/gpu-telemetry:1.0.0` |

**Result**: Full deployment possible with single Helm command (no source code needed):
```bash
helm install telemetry oci://registry-1.docker.io/pp010/gpu-telemetry \
  --version 1.0.0 \
  --namespace gpu-telemetry \
  --create-namespace
```

**Manual Intervention**: Required separate `helm registry login` (Helm uses different auth than Podman).

---

### Kyverno Policies Enabled by Default

**Prompt**:
> "Kyverno Policies is not optional for us, we are enforcing it. Update values.yaml to set policies.enabled: true as default."

**AI Contribution**:
- Changed `policies.enabled` from `false` to `true` in `values.yaml`
- Removed `enable-policies` step from `make up` (no longer needed)
- Updated README to reflect Kyverno is enabled by default

---

### README Improvements

**Prompt 1**:
> "Root README is not correct. It still has make demo details."

**AI Fix**: Removed all `make demo` references, updated Quick Start to show API is ready after `make up`.

**Prompt 2**:
> "This is also wrong: replicas: 3 # Default: 1, Max: 10"

**AI Fix**: Corrected default replica values from 1 to 5 to match `values.yaml`.

**Prompt 3**:
> "Kubernetes Admission Controller (Optional - Kyverno) - also wrong, it's not optional for us"

**AI Fix**: Changed "(Optional - Kyverno)" to "(Kyverno - Enabled by Default)".

**Prompt 4**:
> "gpu-telemetry-pipeline/ - wrong"

**AI Fix**: Changed project structure directory name to `Elastic-GPU-Telemetry/` to match actual repository.

**Prompt 5**:
> "Along with the make commands, all APIs should be also there in README under For Evaluators / Interviewers."

**AI Contribution**:
- Added comprehensive Gateway REST API reference table
- Added query parameters table (start_time, end_time, limit, offset)
- Added supported time formats table
- Added MQ Admin API reference with port-forward instructions
- Added example curl commands with expected responses

---

## Documentation

### README Creation

**Prompt**:
> "Create a root README with all deployment details for interviewers."

**AI Contribution**:
- Comprehensive quick start guide
- Step-by-step evaluation instructions
- Architecture diagrams
- API documentation
- Troubleshooting guide

### DESIGN.md Updates

**Prompt**:
> "Update docs/DESIGN.md with all system design details including DLQ, deployment order, scaling constraints, and Kyverno."

**AI Contribution**:
- Added DLQ architecture section
- Added deployment order with init containers
- Added scaling constraints explanation
- Added admission controller documentation
- Updated all diagrams

---

## Where AI Fell Short

### 1. DLQ Message Cleanup Bug
**Issue**: Messages were being dropped by `MQ_MAX_MESSAGE_AGE` before entering DLQ during extended database outages.

**Why AI Missed It**: The cleanup logic had multiple conditions, and AI didn't anticipate the interaction between retry counting and age-based cleanup.

**Manual Fix Required**: Added conditional check to preserve messages being retried.

---

### 2. Podman/Kind Image Loading
**Issue**: Multiple iterations needed to get images loading correctly with Podman + Kind.

**Why AI Struggled**: 
- Podman's `localhost/` image prefix was unexpected
- Kind's image loading behavior differs between Docker and Podman
- Required trial-and-error debugging

**Resolution**: Eventually solved with `podman save` + `kind load image-archive` approach.

---

### 3. IPv6 vs IPv4 Connection Issues
**Issue**: Integration tests failed on macOS because `localhost` resolved to IPv6.

**Why AI Missed Initially**: Platform-specific networking behavior wasn't immediately obvious.

**Quick Fix Once Identified**: Changed to explicit `127.0.0.1`.

---

### 4. Go Toolchain Version Compatibility
**Issue**: Some dependencies required newer Go versions than initially specified.

**Manual Intervention**: Updated `go.mod` files and fixed protobuf plugin versions.

---

### 5. IDE Build Tag Recognition
**Issue**: Cursor IDE showed "No packages found" for integration test files with `//go:build integration` tag.

**AI Fix**: Created `.vscode/settings.json` with gopls configuration to recognize the tag.

---

## Summary Statistics

| Aspect | AI-Assisted | Manual Intervention |
|--------|-------------|---------------------|
| **Project Structure** | 100% | 0% |
| **Core Code (MQ, Services)** | 95% | 5% (bug fixes) |
| **Unit Tests** | 90% | 10% (timing adjustments) |
| **Integration Tests** | 85% | 15% (platform issues) |
| **Dockerfiles** | 90% | 10% (CSV mounting) |
| **Helm Charts** | 95% | 5% (Podman compat, policies) |
| **Makefile** | 85% | 15% (Podman fixes, port-forward) |
| **Documentation** | 90% | 10% (corrections, API docs location) |

### Overall Assessment

- **~90% of development was AI-assisted**
- **~10% required manual intervention/correction**
- Most manual work was for:
  - Platform-specific issues (Podman, macOS networking)
  - Subtle bugs in complex logic (DLQ cleanup)
  - Tool version compatibility
  - Documentation accuracy (default values, optional vs required, project structure)

### Key Learnings

1. **AI excels at**: Boilerplate code, standard patterns, documentation, test generation
2. **AI needs help with**: Platform-specific quirks, complex state machine edge cases, tool compatibility
3. **Best workflow**: Use AI for initial implementation, then iteratively debug and refine

---

*Document Version: 1.0*  
*Last Updated: January 2026*
