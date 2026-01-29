# PostgreSQL Database

PostgreSQL database for storing GPU telemetry data. Designed to be deployed as a StatefulSet in Kubernetes.

## Overview

This database stores GPU metrics collected from DCGM (Data Center GPU Manager) exporters:
- **Collector** writes telemetry data
- **API Gateway** reads telemetry data for REST APIs

## Prerequisites

You need **one** of the following container engines:
- **Podman** (recommended for macOS): `brew install podman`
- **Docker**: [Install Docker](https://docs.docker.com/get-docker/)

The Makefile auto-detects which engine is available. No `docker-compose` or `podman-compose` needed!

## Schema

### gpu_telemetry Table

| Column | Type | Description |
|--------|------|-------------|
| id | VARCHAR(36) | Primary key (message ID) |
| timestamp | TIMESTAMPTZ | When metric was collected |
| metric_name | VARCHAR(64) | DCGM metric name |
| gpu_index | INTEGER | GPU index (0-7) |
| device | VARCHAR(32) | Linux device (nvidia0) |
| uuid | VARCHAR(64) | Unique GPU identifier |
| model_name | VARCHAR(128) | GPU model |
| hostname | VARCHAR(128) | Host server |
| container | VARCHAR(128) | Container name (optional) |
| pod | VARCHAR(128) | K8s pod name (optional) |
| namespace | VARCHAR(64) | K8s namespace (optional) |
| value | DOUBLE PRECISION | Metric value |
| labels_raw | TEXT | Raw Prometheus labels |
| received_at | TIMESTAMPTZ | When collector received |
| message_id | VARCHAR(36) | MQ message ID |

### Indexes

- `idx_gpu_telemetry_timestamp` - For time-range queries
- `idx_gpu_telemetry_uuid` - For GPU-specific queries
- `idx_gpu_telemetry_uuid_timestamp` - For GPU + time queries
- `idx_gpu_telemetry_hostname` - For host-based queries
- `idx_gpu_telemetry_metric_name` - For metric filtering

### Views

- `gpus` - Unique GPUs with latest info
- `gpus_summary` - Materialized view with GPU statistics

## Quick Start

### Local Development

```bash
# Build the image (first time only)
make build

# Start PostgreSQL
make up

# Wait for it to be ready
make wait

# Check status
make status

# Connect with psql
make psql

# View tables
make tables

# Stop PostgreSQL
make down
```

### One-liner to start fresh
```bash
make build && make up && make wait && make status
```

### Connection Details

| Setting | Value |
|---------|-------|
| Host | localhost |
| Port | 5432 |
| Database | telemetry |
| User | postgres |
| Password | postgres |

**Connection String:**
```
postgres://postgres:postgres@localhost:5432/telemetry?sslmode=disable
```

## Project Structure

```
db/
├── migrations/
│   ├── 001_create_gpu_telemetry.sql   # Main table schema
│   └── 002_create_gpus_view.sql       # Views for API
├── init/
│   └── init.sql                        # Initialization script
├── Dockerfile                          # PostgreSQL image
├── docker-compose.yml                  # Alternative: compose file
├── Makefile                            # Build/run commands (auto-detects podman/docker)
└── README.md
```

## Makefile Targets

Run `make help` to see all available targets. Uses podman or docker automatically.

| Command | Description | Requirements |
|---------|-------------|--------------|
| `make help` | Show all available targets | - |
| `make check-engine` | Verify container engine is available | - |
| **Lifecycle** | | |
| `make build` | Build the PostgreSQL container image | Container runtime |
| `make up` | Start PostgreSQL container | Container runtime |
| `make down` | Stop PostgreSQL container | Container running |
| `make restart` | Restart PostgreSQL container | Container exists |
| `make wait` | Wait for PostgreSQL to be ready (for scripts) | Container running |
| `make clean` | Remove container and ALL data (with confirmation) | Container runtime |
| **Database Access** | | |
| `make psql` | Connect with psql client | Container running |
| `make shell` | Open shell in container | Container running |
| `make logs` | View PostgreSQL logs (follow mode) | Container running |
| `make status` | Check container health and status | Container runtime |
| **Schema** | | |
| `make migrate` | Run migrations manually | Container running |
| `make tables` | List database tables | Container running |
| `make describe` | Describe gpu_telemetry table schema | Container running |
| **Data Queries** | | |
| `make count` | Count total records in gpu_telemetry | Container running |
| `make sample` | Show 10 sample records | Container running |
| `make rows` | Show 50 recent records | Container running |
| `make tail` | Show 20 latest with all columns | Container running |
| `make gpus` | List unique GPUs | Container running |
| `make all` | Show ALL records (use with caution) | Container running |
| `make export` | Export data to CSV file | Container running |

## Kubernetes Deployment

This database is **completely independent** from other services and designed to be deployed as a **StatefulSet**.

### Independence

| Aspect | Description |
|--------|-------------|
| **No shared files** | No code imports from `collector/`, `mq/`, or `streamer/` |
| **Own Dockerfile** | Builds independently |
| **Own Helm chart** | Will be deployed via separate Helm release |
| **Runtime coupling only** | Services connect via connection string (env var) |

### StatefulSet Features

```yaml
# Key StatefulSet configuration
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
spec:
  serviceName: postgres-headless  # Headless service for stable DNS
  replicas: 1                      # Single instance (scale with read replicas if needed)
  selector:
    matchLabels:
      app: postgres
  template:
    spec:
      containers:
        - name: postgres
          image: gpu-telemetry-db:latest
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_DB
              value: telemetry
            - name: POSTGRES_USER
              valueFrom:
                secretKeyRef:
                  name: postgres-secret
                  key: username
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-secret
                  key: password
          volumeMounts:
            - name: postgres-data
              mountPath: /var/lib/postgresql/data
  volumeClaimTemplates:
    - metadata:
        name: postgres-data
      spec:
        accessModes: ["ReadWriteOnce"]
        storageClassName: standard
        resources:
          requests:
            storage: 10Gi
```

### Headless Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: postgres-headless
spec:
  clusterIP: None  # Headless for StatefulSet
  selector:
    app: postgres
  ports:
    - port: 5432
      targetPort: 5432
```

### How Services Connect

Other services (Collector, API Gateway) connect via environment variable:

```yaml
# In Collector/Gateway deployment
env:
  - name: COLLECTOR_DATABASE_URL
    value: "postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres-headless:5432/telemetry?sslmode=disable"
```

Helm charts will be provided in the `helm/` folder.

## Sample Queries

### Get all GPUs
```sql
SELECT DISTINCT uuid, model_name, hostname 
FROM gpu_telemetry;
```

### Get telemetry for a specific GPU
```sql
SELECT * FROM gpu_telemetry 
WHERE uuid = 'GPU-abc123...'
ORDER BY timestamp DESC
LIMIT 100;
```

### Get telemetry with time filter
```sql
SELECT * FROM gpu_telemetry 
WHERE uuid = 'GPU-abc123...'
  AND timestamp >= '2026-01-28T00:00:00Z'
  AND timestamp <= '2026-01-28T23:59:59Z'
ORDER BY timestamp;
```

### Get latest value for each GPU
```sql
SELECT DISTINCT ON (uuid) 
    uuid, model_name, hostname, metric_name, value, timestamp
FROM gpu_telemetry
WHERE metric_name = 'DCGM_FI_DEV_GPU_UTIL'
ORDER BY uuid, timestamp DESC;
```

### Get GPU utilization over time
```sql
SELECT 
    date_trunc('minute', timestamp) as minute,
    uuid,
    AVG(value) as avg_utilization,
    MAX(value) as max_utilization
FROM gpu_telemetry
WHERE metric_name = 'DCGM_FI_DEV_GPU_UTIL'
  AND timestamp >= NOW() - INTERVAL '1 hour'
GROUP BY minute, uuid
ORDER BY minute, uuid;
```

## Backup & Restore

### Backup
```bash
# Using podman (or docker)
podman exec gpu-telemetry-db pg_dump -U postgres telemetry > backup.sql
```

### Restore
```bash
# Using podman (or docker)
podman exec -i gpu-telemetry-db psql -U postgres telemetry < backup.sql
```

## Performance Tuning

For production, consider:

1. **Connection pooling** - Use PgBouncer
2. **Partitioning** - Partition by timestamp for large datasets
3. **Vacuuming** - Regular VACUUM ANALYZE
4. **Indexes** - Add indexes based on query patterns
5. **Replication** - Set up streaming replication for HA
