# GPU Telemetry Pipeline - System Design Document

## Table of Contents
1. [Executive Summary](#1-executive-summary)
2. [System Overview](#2-system-overview)
3. [Architecture](#3-architecture)
4. [Component Design](#4-component-design)
5. [Custom Message Queue Design](#5-custom-message-queue-design)
6. [Dead Letter Queue (DLQ)](#6-dead-letter-queue-dlq)
7. [Data Models](#7-data-models)
8. [API Design](#8-api-design)
9. [Storage Design](#9-storage-design)
10. [Deployment Architecture](#10-deployment-architecture)
11. [Deployment Order & Dependencies](#11-deployment-order--dependencies)
12. [Scaling Strategy](#12-scaling-strategy)
13. [Admission Controller (Kyverno)](#13-admission-controller-kyverno)
14. [Error Handling & Resilience](#14-error-handling--resilience)
15. [Observability](#15-observability)
16. [Security Considerations](#16-security-considerations)
17. [Technology Decisions](#17-technology-decisions)

---

## 1. Executive Summary

This document describes the design of an **Elastic GPU Telemetry Pipeline** that collects, processes, and exposes GPU metrics from an AI cluster. The system uses a custom-built message queue to decouple data producers (streamers) from consumers (collectors), enabling independent scaling and fault tolerance.

### Key Requirements
- Stream GPU telemetry data from CSV files
- Custom message queue implementation (no external MQ libraries)
- Dynamic scaling of streamers and collectors (up to 10 instances)
- REST API for querying telemetry data
- Kubernetes deployment via Helm charts
- Auto-generated OpenAPI specification

---

## 2. System Overview

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
                          │  (External)  │
                          └──────────────┘
```

### Data Flow
1. **Telemetry Streamer** reads GPU metrics from CSV file
2. Streamer publishes metrics to **Custom Message Queue**
3. **Telemetry Collector** subscribes to the queue and receives metrics
4. Collector parses, validates, and persists metrics to **PostgreSQL**
5. **API Gateway** serves REST endpoints to query stored telemetry

---

## 3. Architecture

### 3.1 High-Level Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                         KUBERNETES CLUSTER                             │
├────────────────────────────────────────────────────────────────────────┤
│                                                                        │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │                    Namespace: gpu-telemetry                      │  │
│  │                                                                  │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │  │
│  │  │  Streamer    │  │  Streamer    │  │  Streamer    │          │  │
│  │  │  Pod 1       │  │  Pod 2       │  │  Pod N       │          │  │
│  │  │  (Replica)   │  │  (Replica)   │  │  (Replica)   │          │  │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘          │  │
│  │         │                 │                 │                   │  │
│  │         └────────────────┬┴─────────────────┘                   │  │
│  │                          ▼                                      │  │
│  │              ┌───────────────────────┐                          │  │
│  │              │   Message Queue       │                          │  │
│  │              │   Service (Broker)    │                          │  │
│  │              │   ┌─────────────────┐ │                          │  │
│  │              │   │ Topic: gpu-     │ │                          │  │
│  │              │   │ telemetry       │ │                          │  │
│  │              │   └─────────────────┘ │                          │  │
│  │              └───────────┬───────────┘                          │  │
│  │                          │                                      │  │
│  │         ┌────────────────┼────────────────┐                     │  │
│  │         ▼                ▼                ▼                     │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │  │
│  │  │  Collector   │  │  Collector   │  │  Collector   │          │  │
│  │  │  Pod 1       │  │  Pod 2       │  │  Pod N       │          │  │
│  │  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘          │  │
│  │         │                 │                 │                   │  │
│  │         └────────────────┬┴─────────────────┘                   │  │
│  │                          ▼                                      │  │
│  │              ┌───────────────────────┐                          │  │
│  │              │     PostgreSQL        │                          │  │
│  │              │     (StatefulSet)     │                          │  │
│  │              └───────────┬───────────┘                          │  │
│  │                          │                                      │  │
│  │                          ▼                                      │  │
│  │              ┌───────────────────────┐                          │  │
│  │              │     API Gateway       │◄──── Ingress/Service     │  │
│  │              │     (Deployment)      │                          │  │
│  │              └───────────────────────┘                          │  │
│  │                                                                  │  │
│  └─────────────────────────────────────────────────────────────────┘  │
│                                                                        │
└────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Component Interaction

```
┌──────────────┐         ┌──────────────┐         ┌──────────────┐
│   Streamer   │  gRPC   │   Message    │  gRPC   │  Collector   │
│              ├────────►│   Queue      ├────────►│              │
│  (Producer)  │ Publish │   Broker     │Subscribe│  (Consumer)  │
│              │         │   :8081      │  :8081  │              │
└──────────────┘         └──────────────┘         └──────┬───────┘
                               │                         │
                               │ HTTP :8082              │ SQL
                               │ (Admin APIs)            ▼
┌──────────────┐         ┌─────┴────────┐         ┌──────────────┐
│   Client     │  HTTP   │     API      │  SQL    │  PostgreSQL  │
│              │◄───────►│   Gateway    │◄───────►│              │
│              │  REST   │    :8080     │         │    :5432     │
└──────────────┘         └──────────────┘         └──────────────┘

MQ Ports:
  - 8081: gRPC (Publish/Subscribe - used by Streamer/Collector)
  - 8082: HTTP (Admin APIs, Swagger, Health checks)
```

---

## 4. Component Design

### 4.1 Telemetry Streamer

**Purpose**: Read GPU telemetry from CSV and publish to message queue.

```
┌─────────────────────────────────────────────────────────────────┐
│                      TELEMETRY STREAMER                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────────────┐ │
│  │  CSV Reader │───►│  Parser     │───►│  MQ Client          │ │
│  │             │    │             │    │  (Publisher)        │ │
│  │  - Read     │    │  - Validate │    │                     │ │
│  │  - Loop     │    │  - Transform│    │  - Connect          │ │
│  │  - Buffer   │    │  - Enrich   │    │  - Publish          │ │
│  │             │    │    (add ts) │    │  - Retry            │ │
│  └─────────────┘    └─────────────┘    └─────────────────────┘ │
│                                                                 │
│  Configuration:                                                 │
│  - CSV_FILE_PATH: /data/metrics.csv                            │
│  - MQ_BROKER_ADDR: mq-broker:8081 (gRPC)                       │
│  - TOPIC: gpu-telemetry                                        │
│  - STREAM_INTERVAL: 100ms                                      │
│  - BATCH_SIZE: 10                                              │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Key Features**:
- Reads CSV file line by line
- Loops continuously to simulate real-time streaming
- Adds processing timestamp to each record
- Publishes to message queue with retry logic
- Supports graceful shutdown
- Configurable streaming interval

**Pseudocode**:
```go
func (s *Streamer) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return nil
        default:
            record, err := s.csvReader.ReadNext()
            if err == io.EOF {
                s.csvReader.Reset() // Loop back
                continue
            }
            
            telemetry := s.parser.Parse(record)
            telemetry.Timestamp = time.Now()
            
            err = s.mqClient.Publish(ctx, s.topic, telemetry)
            if err != nil {
                s.handleError(err)
            }
            
            time.Sleep(s.streamInterval)
        }
    }
}
```

---

### 4.2 Telemetry Collector

**Purpose**: Consume telemetry from message queue and persist to database.

```
┌─────────────────────────────────────────────────────────────────┐
│                     TELEMETRY COLLECTOR                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐    ┌─────────────┐    ┌─────────────────┐ │
│  │  MQ Client      │───►│  Processor  │───►│  Repository     │ │
│  │  (Subscriber)   │    │             │    │                 │ │
│  │                 │    │  - Validate │    │  - Insert       │ │
│  │  - Subscribe    │    │  - Dedupe   │    │  - Batch Write  │ │
│  │  - Long Poll    │    │  - Buffer   │    │  - Transaction  │ │
│  │  - Ack/Nack     │    │             │    │                 │ │
│  └─────────────────┘    └─────────────┘    └─────────────────┘ │
│                                                                 │
│  Configuration:                                                 │
│  - MQ_BROKER_ADDR: mq-broker:8081 (gRPC)                       │
│  - TOPIC: gpu-telemetry                                        │
│  - CONSUMER_GROUP: collectors                                  │
│  - DB_CONNECTION_STRING: postgres://...                        │
│  - BATCH_SIZE: 100                                             │
│  - FLUSH_INTERVAL: 1s                                          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Key Features**:
- Subscribes to message queue topic
- Part of consumer group for load balancing
- Batches writes for efficiency
- Acknowledges messages after successful persistence
- Handles duplicates gracefully

**Pseudocode**:
```go
func (c *Collector) Run(ctx context.Context) error {
    msgChan := c.mqClient.Subscribe(ctx, c.topic, c.consumerGroup)
    batch := make([]Telemetry, 0, c.batchSize)
    ticker := time.NewTicker(c.flushInterval)
    
    for {
        select {
        case <-ctx.Done():
            return c.flush(batch)
        case msg := <-msgChan:
            telemetry, err := c.processor.Process(msg)
            if err != nil {
                msg.Nack()
                continue
            }
            batch = append(batch, telemetry)
            if len(batch) >= c.batchSize {
                c.flush(batch)
                batch = batch[:0]
            }
            msg.Ack()
        case <-ticker.C:
            if len(batch) > 0 {
                c.flush(batch)
                batch = batch[:0]
            }
        }
    }
}
```

---

### 4.3 API Gateway

**Purpose**: Expose REST API for querying telemetry data.

```
┌─────────────────────────────────────────────────────────────────┐
│                        API GATEWAY                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    HTTP Router                           │   │
│  │                   (chi / gin)                            │   │
│  └─────────────────────────────────────────────────────────┘   │
│           │                    │                    │           │
│           ▼                    ▼                    ▼           │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │  GET /api/v1/   │  │  GET /api/v1/   │  │  GET /health    │ │
│  │  gpus           │  │  gpus/{id}/     │  │                 │ │
│  │                 │  │  telemetry      │  │  GET /ready     │ │
│  │  List all GPUs  │  │                 │  │                 │ │
│  │                 │  │  Query params:  │  │  GET /metrics   │ │
│  │                 │  │  - start_time   │  │                 │ │
│  │                 │  │  - end_time     │  │                 │ │
│  └────────┬────────┘  └────────┬────────┘  └─────────────────┘ │
│           │                    │                                │
│           └─────────┬──────────┘                                │
│                     ▼                                           │
│           ┌─────────────────┐                                   │
│           │   Repository    │                                   │
│           │   (Database)    │                                   │
│           └─────────────────┘                                   │
│                                                                 │
│  Features:                                                      │
│  - OpenAPI spec auto-generation (swaggo/swag)                  │
│  - Request validation                                          │
│  - Pagination support                                          │
│  - Error handling middleware                                   │
│  - Logging middleware                                          │
│  - CORS support                                                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**API Endpoints**:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/gpus` | List all GPUs with telemetry |
| GET | `/api/v1/gpus/{id}/telemetry` | Get telemetry for specific GPU |
| GET | `/api/v1/gpus/{id}/telemetry?start_time=X&end_time=Y` | Filtered telemetry |
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |
| GET | `/swagger/*` | Swagger UI |

---

## 5. Custom Message Queue Design

### 5.1 Overview

The custom message queue is the heart of this system. It provides:
- **Topic-based pub/sub**
- **Consumer groups** for load balancing
- **Dead Letter Queue (DLQ)** for failed messages with auto-replay
- **At-least-once delivery** semantics
- **Dual protocol**: gRPC (inter-service, high performance) + HTTP (admin APIs)

### 5.2 Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        MESSAGE QUEUE BROKER                             │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                         HTTP Server                                │ │
│  │                        (Port 8080)                                 │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                               │                                         │
│           ┌───────────────────┼───────────────────┐                    │
│           ▼                   ▼                   ▼                    │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐          │
│  │ POST /publish   │ │ POST /subscribe │ │ POST /ack       │          │
│  │                 │ │                 │ │                 │          │
│  │ Publish message │ │ Long-poll for   │ │ Acknowledge     │          │
│  │ to topic        │ │ messages        │ │ message         │          │
│  └────────┬────────┘ └────────┬────────┘ └────────┬────────┘          │
│           │                   │                   │                    │
│           └───────────────────┼───────────────────┘                    │
│                               ▼                                         │
│  ┌───────────────────────────────────────────────────────────────────┐ │
│  │                       BROKER CORE                                  │ │
│  │                                                                    │ │
│  │  ┌─────────────────────────────────────────────────────────────┐  │ │
│  │  │                    Topic Manager                             │  │ │
│  │  │                                                              │  │ │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │  │ │
│  │  │  │   Topic A   │  │   Topic B   │  │   Topic N   │         │  │ │
│  │  │  │             │  │             │  │             │         │  │ │
│  │  │  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │         │  │ │
│  │  │  │ │ Queue   │ │  │ │ Queue   │ │  │ │ Queue   │ │         │  │ │
│  │  │  │ │ [][][]  │ │  │ │ [][][]  │ │  │ │ [][][]  │ │         │  │ │
│  │  │  │ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │         │  │ │
│  │  │  │             │  │             │  │             │         │  │ │
│  │  │  │ Consumers:  │  │ Consumers:  │  │ Consumers:  │         │  │ │
│  │  │  │ - Group A   │  │ - Group X   │  │ - Group Y   │         │  │ │
│  │  │  │ - Group B   │  │             │  │             │         │  │ │
│  │  │  └─────────────┘  └─────────────┘  └─────────────┘         │  │ │
│  │  │                                                              │  │ │
│  │  └─────────────────────────────────────────────────────────────┘  │ │
│  │                                                                    │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### 5.3 Core Data Structures

```go
// Message represents a single message in the queue
type Message struct {
    ID        string    `json:"id"`
    Topic     string    `json:"topic"`
    Payload   []byte    `json:"payload"`
    Timestamp time.Time `json:"timestamp"`
    Attempts  int       `json:"attempts"`
}

// Topic manages messages and consumer groups
type Topic struct {
    Name           string
    Messages       *ring.Buffer      // Circular buffer for messages
    ConsumerGroups map[string]*ConsumerGroup
    mu             sync.RWMutex
}

// ConsumerGroup manages multiple consumers sharing work
type ConsumerGroup struct {
    Name           string
    Consumers      map[string]*Consumer
    PendingAcks    map[string]*PendingMessage  // msgID -> pending
    LastOffset     int64
    mu             sync.RWMutex
}

// Consumer represents a single consumer connection
type Consumer struct {
    ID          string
    GroupName   string
    TopicName   string
    MessageChan chan *Message
    LastSeen    time.Time
}
```

### 5.4 API Specification

#### Publish Message
```
POST /api/v1/topics/{topic}/messages

Request:
{
    "payload": "<base64-encoded-data>"
}

Response (201 Created):
{
    "message_id": "msg-uuid-here",
    "topic": "gpu-telemetry",
    "timestamp": "2025-01-22T10:00:00Z"
}
```

#### Subscribe (Long-Poll)
```
POST /api/v1/topics/{topic}/subscribe

Request:
{
    "consumer_group": "collectors",
    "consumer_id": "collector-1",
    "timeout_ms": 30000,
    "max_messages": 10
}

Response (200 OK):
{
    "messages": [
        {
            "id": "msg-uuid",
            "payload": "<base64-encoded-data>",
            "timestamp": "2025-01-22T10:00:00Z"
        }
    ]
}
```

#### Acknowledge Message
```
POST /api/v1/topics/{topic}/ack

Request:
{
    "consumer_group": "collectors",
    "message_ids": ["msg-uuid-1", "msg-uuid-2"]
}

Response (200 OK):
{
    "acknowledged": 2
}
```

### 5.5 Consumer Group Load Balancing

```
                    ┌─────────────────┐
                    │     Topic       │
                    │  gpu-telemetry  │
                    │                 │
                    │  Messages:      │
                    │  [1][2][3][4]   │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
              ▼              ▼              ▼
     ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
     │ Consumer    │ │ Consumer    │ │ Consumer    │
     │ Group:      │ │ Group:      │ │ Group:      │
     │ collectors  │ │ collectors  │ │ collectors  │
     │             │ │             │ │             │
     │ Consumer 1  │ │ Consumer 2  │ │ Consumer 3  │
     │ Gets: [1,4] │ │ Gets: [2]   │ │ Gets: [3]   │
     └─────────────┘ └─────────────┘ └─────────────┘
     
     Round-robin distribution within consumer group
```

### 5.6 Message Lifecycle

```
    ┌──────────┐
    │ PENDING  │◄────────────────────────────┐
    │          │                              │
    └────┬─────┘                              │
         │ Publish                            │
         ▼                                    │
    ┌──────────┐                              │
    │  QUEUED  │                              │
    │          │                              │
    └────┬─────┘                              │
         │ Delivered to Consumer              │
         ▼                                    │
    ┌──────────┐         Timeout/Nack         │
    │ DELIVERED│──────────────────────────────┘
    │ (Pending │         (Requeue)
    │   Ack)   │
    └────┬─────┘
         │ Ack received
         ▼
    ┌──────────┐
    │   ACKED  │
    │ (Removed)│
    └──────────┘
```

---

## 6. Dead Letter Queue (DLQ)

### 6.1 Overview

The DLQ handles messages that fail processing (e.g., database down). Instead of losing messages or infinite retries, failed messages are moved to a DLQ for later replay.

### 6.2 DLQ Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    MESSAGE LIFECYCLE WITH DLQ                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐   │
│  │ PENDING  │────▶│ DELIVERED│────▶│  ACKED   │────▶│ REMOVED  │   │
│  └──────────┘     └────┬─────┘     └──────────┘     └──────────┘   │
│                        │                                             │
│                        │ NACK (failure)                              │
│                        ▼                                             │
│                   ┌──────────┐                                       │
│                   │  RETRY   │ (retry_count++)                       │
│                   └────┬─────┘                                       │
│                        │                                             │
│                        │ retry_count > MAX_RETRIES                   │
│                        ▼                                             │
│                   ┌──────────┐                                       │
│                   │   DLQ    │ (dlq_retry_count = 0)                │
│                   └────┬─────┘                                       │
│                        │                                             │
│                        │ After DLQ_RETRY_DELAY (5 min)              │
│                        ▼                                             │
│                   ┌──────────┐                                       │
│                   │ REPLAYED │ Back to main topic                    │
│                   │ TO TOPIC │ (dlq_retry_count++)                  │
│                   └────┬─────┘                                       │
│                        │                                             │
│                        │ dlq_retry_count > DLQ_MAX_RETRIES          │
│                        ▼                                             │
│                   ┌──────────┐                                       │
│                   │   DEAD   │ Requires manual intervention          │
│                   └──────────┘                                       │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 6.3 Why Auto-Replay Instead of Direct DLQ Consumer?

| Problem | Direct DLQ Consumer | Auto-Replay Design |
|---------|--------------------|--------------------|
| **Infinite loop** | If DB still down, message fails again → Where does it go? | Message goes back to DLQ with `dlq_retry_count++`, stops after max retries |
| **Retry counting** | Complex to track retries across two subscriptions | Built-in `retry_count` and `dlq_retry_count` per message |
| **Dead state** | Need separate logic to stop infinite retries | Automatic `Dead` state after N replays |
| **Code complexity** | Collector handles 2 topics with different logic | Single consumer path, same code for all messages |

### 6.4 DLQ Configuration

```yaml
MQ_DLQ_ENABLED: "true"       # Enable DLQ functionality
MQ_MAX_RETRIES: "3"          # Retries before moving to DLQ
MQ_DLQ_MAX_RETRIES: "3"      # DLQ replays before marking Dead
MQ_DLQ_RETRY_DELAY: "5m"     # Wait time between DLQ replays
```

### 6.5 DLQ Admin APIs

| Endpoint | Description |
|----------|-------------|
| `GET /api/v1/topics/{topic}/dlq` | List messages in DLQ |
| `GET /api/v1/topics/{topic}/dlq/count` | Count DLQ messages |
| `POST /api/v1/topics/{topic}/dlq/replay` | Manually trigger replay |
| `DELETE /api/v1/topics/{topic}/dlq/{id}` | Remove message from DLQ |
| `GET /api/v1/topics/{topic}/dead` | List dead messages |

### 6.6 DLQ Timeline Example

```
Time  | Event
──────┼──────────────────────────────────────────────
00:00 | Message published, DB is down
00:00 | Collector NACKs → retry_count=1
00:01 | Retry 2 fails → retry_count=2
00:02 | Retry 3 fails → retry_count=3
00:03 | Moved to DLQ (max retries exceeded)
00:08 | Auto-replay #1 → dlq_retry_count=1, fails
00:13 | Auto-replay #2 → dlq_retry_count=2, fails
00:18 | Auto-replay #3 → dlq_retry_count=3, fails
00:23 | Marked as DEAD (max DLQ retries exceeded)
```

---

## 7. Data Models

### 6.1 GPU Telemetry Record

Based on DCGM (Data Center GPU Manager) metrics:

```go
type GPUTelemetry struct {
    ID                  int64     `json:"id" db:"id"`
    GPUUUID             string    `json:"gpu_uuid" db:"gpu_uuid"`
    GPUIndex            int       `json:"gpu_index" db:"gpu_index"`
    Hostname            string    `json:"hostname" db:"hostname"`
    Timestamp           time.Time `json:"timestamp" db:"timestamp"`
    
    // Utilization Metrics
    GPUUtilization      float64   `json:"gpu_utilization" db:"gpu_utilization"`
    MemoryUtilization   float64   `json:"memory_utilization" db:"memory_utilization"`
    
    // Memory Metrics
    MemoryUsed          int64     `json:"memory_used" db:"memory_used"`
    MemoryFree          int64     `json:"memory_free" db:"memory_free"`
    MemoryTotal         int64     `json:"memory_total" db:"memory_total"`
    
    // Temperature & Power
    Temperature         float64   `json:"temperature" db:"temperature"`
    PowerUsage          float64   `json:"power_usage" db:"power_usage"`
    PowerLimit          float64   `json:"power_limit" db:"power_limit"`
    
    // Performance
    SMClock             int       `json:"sm_clock" db:"sm_clock"`
    MemoryClock         int       `json:"memory_clock" db:"memory_clock"`
    
    // PCIe Metrics
    PCIeTxThroughput    int64     `json:"pcie_tx_throughput" db:"pcie_tx_throughput"`
    PCIeRxThroughput    int64     `json:"pcie_rx_throughput" db:"pcie_rx_throughput"`
}
```

### 6.2 API Response Models

```go
// GPU represents a GPU device
type GPU struct {
    UUID     string `json:"uuid"`
    Index    int    `json:"index"`
    Hostname string `json:"hostname"`
}

// GPUListResponse for GET /api/v1/gpus
type GPUListResponse struct {
    GPUs  []GPU `json:"gpus"`
    Count int   `json:"count"`
}

// TelemetryResponse for GET /api/v1/gpus/{id}/telemetry
type TelemetryResponse struct {
    GPUUUID    string         `json:"gpu_uuid"`
    Telemetry  []GPUTelemetry `json:"telemetry"`
    Count      int            `json:"count"`
    StartTime  *time.Time     `json:"start_time,omitempty"`
    EndTime    *time.Time     `json:"end_time,omitempty"`
}
```

---

## 8. API Design

### 7.1 OpenAPI Specification (Auto-generated)

Using `swaggo/swag` for auto-generation:

```go
// @title GPU Telemetry API
// @version 1.0
// @description REST API for GPU telemetry data
// @host localhost:8080
// @BasePath /api/v1

// ListGPUs godoc
// @Summary List all GPUs
// @Description Get a list of all GPUs that have telemetry data
// @Tags gpus
// @Accept json
// @Produce json
// @Success 200 {object} GPUListResponse
// @Router /gpus [get]
func (h *Handler) ListGPUs(w http.ResponseWriter, r *http.Request) {
    // Implementation
}

// GetGPUTelemetry godoc
// @Summary Get GPU telemetry
// @Description Get telemetry data for a specific GPU
// @Tags gpus
// @Accept json
// @Produce json
// @Param id path string true "GPU UUID"
// @Param start_time query string false "Start time (RFC3339)"
// @Param end_time query string false "End time (RFC3339)"
// @Success 200 {object} TelemetryResponse
// @Failure 404 {object} ErrorResponse
// @Router /gpus/{id}/telemetry [get]
func (h *Handler) GetGPUTelemetry(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

### 7.2 Error Responses

```go
type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
    Code    int    `json:"code"`
}

// Standard error codes
// 400 - Bad Request (invalid parameters)
// 404 - Not Found (GPU not found)
// 500 - Internal Server Error
```

---

## 9. Storage Design

### 8.1 PostgreSQL Schema

```sql
-- GPUs table (denormalized from telemetry for quick lookups)
CREATE TABLE gpus (
    uuid VARCHAR(64) PRIMARY KEY,
    index INTEGER NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    first_seen TIMESTAMP WITH TIME ZONE NOT NULL,
    last_seen TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_gpus_hostname ON gpus(hostname);

-- Telemetry table (time-series data)
CREATE TABLE gpu_telemetry (
    id BIGSERIAL PRIMARY KEY,
    gpu_uuid VARCHAR(64) NOT NULL REFERENCES gpus(uuid),
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    
    -- Utilization
    gpu_utilization DOUBLE PRECISION,
    memory_utilization DOUBLE PRECISION,
    
    -- Memory
    memory_used BIGINT,
    memory_free BIGINT,
    memory_total BIGINT,
    
    -- Temperature & Power
    temperature DOUBLE PRECISION,
    power_usage DOUBLE PRECISION,
    power_limit DOUBLE PRECISION,
    
    -- Clocks
    sm_clock INTEGER,
    memory_clock INTEGER,
    
    -- PCIe
    pcie_tx_throughput BIGINT,
    pcie_rx_throughput BIGINT,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for query performance
CREATE INDEX idx_telemetry_gpu_uuid ON gpu_telemetry(gpu_uuid);
CREATE INDEX idx_telemetry_timestamp ON gpu_telemetry(timestamp);
CREATE INDEX idx_telemetry_gpu_timestamp ON gpu_telemetry(gpu_uuid, timestamp);

-- Partitioning by time (optional, for scale)
-- CREATE TABLE gpu_telemetry_y2025m01 PARTITION OF gpu_telemetry
--     FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
```

### 8.2 Repository Pattern

```go
type TelemetryRepository interface {
    // GPU operations
    UpsertGPU(ctx context.Context, gpu *GPU) error
    ListGPUs(ctx context.Context) ([]GPU, error)
    GetGPU(ctx context.Context, uuid string) (*GPU, error)
    
    // Telemetry operations
    InsertTelemetry(ctx context.Context, telemetry *GPUTelemetry) error
    InsertTelemetryBatch(ctx context.Context, telemetry []GPUTelemetry) error
    GetTelemetryByGPU(ctx context.Context, gpuUUID string, 
                       startTime, endTime *time.Time) ([]GPUTelemetry, error)
}
```

---

## 10. Deployment Architecture

### 9.1 Kubernetes Resources

```yaml
# Namespace
apiVersion: v1
kind: Namespace
metadata:
  name: gpu-telemetry

---
# Components
┌──────────────────────────────────────────────────────────┐
│                     Deployments                          │
├──────────────────────────────────────────────────────────┤
│  • mq-broker (replicas: 1)                              │
│  • telemetry-streamer (replicas: 1-10, HPA)             │
│  • telemetry-collector (replicas: 1-10, HPA)            │
│  • api-gateway (replicas: 2-3, HPA)                     │
├──────────────────────────────────────────────────────────┤
│                     StatefulSets                         │
├──────────────────────────────────────────────────────────┤
│  • postgresql (replicas: 1)                             │
├──────────────────────────────────────────────────────────┤
│                      Services                            │
├──────────────────────────────────────────────────────────┤
│  • mq-broker-svc (ClusterIP)                            │
│  • api-gateway-svc (ClusterIP / LoadBalancer)           │
│  • postgresql-svc (ClusterIP)                           │
├──────────────────────────────────────────────────────────┤
│                     ConfigMaps                           │
├──────────────────────────────────────────────────────────┤
│  • app-config (shared configuration)                    │
│  • telemetry-data (CSV file mount)                      │
├──────────────────────────────────────────────────────────┤
│                      Secrets                             │
├──────────────────────────────────────────────────────────┤
│  • db-credentials                                       │
└──────────────────────────────────────────────────────────┘
```

### 9.2 Helm Chart Structure

```
helm/
└── gpu-telemetry/
    ├── Chart.yaml
    ├── values.yaml
    ├── templates/
    │   ├── _helpers.tpl
    │   ├── namespace.yaml
    │   ├── configmap.yaml
    │   ├── secret.yaml
    │   ├── mq-broker/
    │   │   ├── deployment.yaml
    │   │   └── service.yaml
    │   ├── streamer/
    │   │   ├── deployment.yaml
    │   │   └── hpa.yaml
    │   ├── collector/
    │   │   ├── deployment.yaml
    │   │   └── hpa.yaml
    │   ├── api-gateway/
    │   │   ├── deployment.yaml
    │   │   ├── service.yaml
    │   │   └── hpa.yaml
    │   └── postgresql/
    │       ├── statefulset.yaml
    │       ├── service.yaml
    │       └── pvc.yaml
    └── charts/
```

### 9.3 Resource Requirements

| Component | CPU Request | CPU Limit | Memory Request | Memory Limit |
|-----------|-------------|-----------|----------------|--------------|
| MQ Broker | 100m | 500m | 128Mi | 512Mi |
| Streamer | 50m | 200m | 64Mi | 256Mi |
| Collector | 100m | 300m | 128Mi | 512Mi |
| API Gateway | 100m | 500m | 128Mi | 512Mi |
| PostgreSQL | 200m | 1000m | 256Mi | 1Gi |

---

## 11. Deployment Order & Dependencies

### 11.1 Service Dependencies

Services are deployed simultaneously but use **init containers** to wait for dependencies:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    DEPLOYMENT ORDER                                  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   Phase 1: Infrastructure (no dependencies)                         │
│   ┌──────────────┐    ┌──────────────┐                              │
│   │  PostgreSQL  │    │      MQ      │                              │
│   │ (StatefulSet)│    │ (Deployment) │                              │
│   └──────────────┘    └──────────────┘                              │
│          │                   │                                       │
│          ▼                   ▼                                       │
│   Phase 2: Services (wait via init containers)                      │
│                                                                      │
│   ┌─────────────────────────────────────────┐                       │
│   │  Gateway         waits for: PostgreSQL  │                       │
│   └─────────────────────────────────────────┘                       │
│                                                                      │
│   ┌─────────────────────────────────────────┐                       │
│   │  Collector       waits for: PostgreSQL  │                       │
│   │                            + MQ         │                       │
│   └─────────────────────────────────────────┘                       │
│                                                                      │
│   ┌─────────────────────────────────────────┐                       │
│   │  Streamer        waits for: MQ          │                       │
│   └─────────────────────────────────────────┘                       │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 11.2 Init Container Configuration

```yaml
# Collector deployment - waits for both PostgreSQL and MQ
initContainers:
  - name: wait-for-postgres
    image: busybox:1.36
    command: ['sh', '-c', 'until nc -z postgres 5432; do echo waiting for postgres; sleep 2; done']
  - name: wait-for-mq
    image: busybox:1.36
    command: ['sh', '-c', 'until nc -z mq 8081; do echo waiting for mq; sleep 2; done']
```

### 11.3 Startup Timeline

```
Time ──────────────────────────────────────────────────────▶

PostgreSQL  ████████████████████████████████████████████████▶ (ready ~15s)
MQ          ██████████████████████████████████████████████████▶ (ready ~10s)
                         │
                         ▼ (dependencies ready)
Gateway     ░░░░░░░░░░░░░████████████████████████████████████▶
            (waiting)

Streamer    ░░░░░░░░░░░░░████████████████████████████████████▶
            (waiting)

Collector   ░░░░░░░░░░░░░░░░░████████████████████████████████▶
            (waiting for both)

░░░ = init container waiting
███ = running
```

### 11.4 Dependency Matrix

| Component | PostgreSQL | MQ | Init Container |
|-----------|------------|-----|----------------|
| PostgreSQL | - | - | None |
| MQ | - | - | None |
| Gateway | ✅ | - | `wait-for-postgres` |
| Streamer | - | ✅ | `wait-for-mq` |
| Collector | ✅ | ✅ | `wait-for-postgres`, `wait-for-mq` |

---

## 12. Scaling Strategy

### 12.1 Scaling Constraints

| Component | Min | Max | Can Scale? | Reason |
|-----------|-----|-----|------------|--------|
| **Streamer** | 1 | 10 | ✅ Yes | Stateless, each reads CSV and publishes |
| **Collector** | 1 | 10 | ✅ Yes | Stateless, MQ distributes via consumer groups |
| **Gateway** | 1 | 10 | ✅ Yes | Stateless, all read from shared PostgreSQL |
| **MQ Broker** | 1 | 1 | ❌ No | In-memory state, cannot scale horizontally |
| **PostgreSQL** | 1 | 1 | ❌ No | Single-writer database |

### 12.2 Why MQ Cannot Scale Horizontally

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

### 12.3 Why PostgreSQL Cannot Scale Horizontally

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

### 12.4 Scaling Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    SCALING ARCHITECTURE                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Load Increase                                              │
│       │                                                     │
│       ▼                                                     │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐                │
│  │Streamer │    │Streamer │    │Streamer │  ← Scale 1-10  │
│  │   1     │    │   2     │    │   N     │                │
│  └────┬────┘    └────┬────┘    └────┬────┘                │
│       │              │              │                       │
│       └──────────────┼──────────────┘                       │
│                      ▼                                      │
│              ┌───────────────┐                              │
│              │  MQ Broker    │  ← SINGLE INSTANCE          │
│              │  (in-memory)  │    (cannot scale!)          │
│              └───────┬───────┘                              │
│                      │                                      │
│       ┌──────────────┼──────────────┐                       │
│       │              │              │                       │
│       ▼              ▼              ▼                       │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐                │
│  │Collector│    │Collector│    │Collector│  ← Scale 1-10  │
│  │   1     │    │   2     │    │   N     │                │
│  └────┬────┘    └────┬────┘    └────┬────┘                │
│       │              │              │                       │
│       └──────────────┼──────────────┘                       │
│                      ▼                                      │
│              ┌───────────────┐                              │
│              │  PostgreSQL   │  ← SINGLE INSTANCE          │
│              │               │    (single-writer)          │
│              └───────────────┘                              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 12.5 PostgreSQL Concurrency with Multiple Collectors/Gateways

PostgreSQL handles 10 Collectors + 10 Gateways well because:

| Aspect | Behavior |
|--------|----------|
| **Concurrent Reads** | Excellent - PostgreSQL's strength with MVCC |
| **Concurrent Writes** | Good - Each INSERT is independent, minimal contention |
| **Connections** | 20 total (well under default limit of 100) |
| **Read/Write Mix** | Writers don't block readers (MVCC) |

---

## 13. Admission Controller (Kyverno)

### 13.1 Overview

Scaling limits are enforced at two levels:
1. **Makefile validation** - Prevents `make scale-*` commands from exceeding limits
2. **Kyverno policies** - Enforces limits at Kubernetes API level (blocks `kubectl scale`)

### 13.2 Kyverno Policies

```yaml
# Policy: Limit scalable replicas (max 10)
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: limit-scalable-replicas
spec:
  validationFailureAction: Enforce
  rules:
    - name: limit-streamer-replicas
      match:
        resources:
          kinds: [Deployment]
          names: [streamer]
      validate:
        message: "Streamer replicas must be between 1 and 10"
        pattern:
          spec:
            replicas: "1-10"
```

```yaml
# Policy: Limit singleton replicas (max 1)
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: limit-singleton-replicas
spec:
  validationFailureAction: Enforce
  rules:
    - name: limit-mq-replicas
      match:
        resources:
          kinds: [Deployment]
          names: [mq]
      validate:
        message: "MQ must have exactly 1 replica"
        pattern:
          spec:
            replicas: 1
```

### 13.3 Policy Enforcement Examples

```bash
# Allowed: Scale streamer to 5
kubectl scale deployment streamer --replicas=5 -n gpu-telemetry
# ✓ deployment.apps/streamer scaled

# Blocked: Scale streamer beyond limit
kubectl scale deployment streamer --replicas=15 -n gpu-telemetry
# Error: Streamer replicas must be between 1 and 10

# Blocked: Scale MQ (singleton)
kubectl scale deployment mq --replicas=2 -n gpu-telemetry
# Error: MQ must have exactly 1 replica (in-memory broker cannot scale)

# Blocked: Scale PostgreSQL (singleton)
kubectl scale statefulset postgres --replicas=2 -n gpu-telemetry
# Error: PostgreSQL must have exactly 1 replica (single-writer database)
```

### 13.4 Installing Kyverno

```bash
# Kyverno is installed automatically with `make up`
# Or manually:
make install-kyverno
make enable-policies
make test-policies
```

---

## 14. Error Handling & Resilience

### 14.1 Retry Strategy

```go
type RetryConfig struct {
    MaxRetries     int           // Maximum retry attempts
    InitialBackoff time.Duration // Initial backoff duration
    MaxBackoff     time.Duration // Maximum backoff duration
    Multiplier     float64       // Backoff multiplier
}

// Default configuration
var DefaultRetryConfig = RetryConfig{
    MaxRetries:     5,
    InitialBackoff: 100 * time.Millisecond,
    MaxBackoff:     30 * time.Second,
    Multiplier:     2.0,
}
```

### 14.2 Circuit Breaker Pattern

```
     ┌─────────────────────────────────────────┐
     │           Circuit Breaker               │
     │                                         │
     │  ┌─────────┐  failures  ┌─────────┐    │
     │  │ CLOSED  │──────────►│  OPEN   │    │
     │  │         │  > threshold         │    │
     │  └────┬────┘           └────┬────┘    │
     │       │                     │          │
     │       │ success             │ timeout  │
     │       │                     ▼          │
     │       │              ┌───────────┐     │
     │       │              │HALF-OPEN  │     │
     │       │              └─────┬─────┘     │
     │       │                    │           │
     │       │◄───────────────────┘           │
     │               success                  │
     └─────────────────────────────────────────┘
```

### 14.3 Graceful Shutdown

```go
func (s *Server) GracefulShutdown(timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    // 1. Stop accepting new requests
    s.server.SetKeepAlivesEnabled(false)

    // 2. Wait for in-flight requests
    if err := s.server.Shutdown(ctx); err != nil {
        return err
    }

    // 3. Close database connections
    if err := s.db.Close(); err != nil {
        return err
    }

    // 4. Close MQ connections
    if err := s.mqClient.Close(); err != nil {
        return err
    }

    return nil
}
```

---

## 15. Observability

### 15.1 Logging

```go
// Structured logging with zerolog
log.Info().
    Str("component", "streamer").
    Str("topic", "gpu-telemetry").
    Int("batch_size", len(batch)).
    Dur("duration", elapsed).
    Msg("Published telemetry batch")
```

### 15.2 Metrics (Prometheus)

```go
// Key metrics to expose
var (
    messagesPublished = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mq_messages_published_total",
            Help: "Total messages published",
        },
        []string{"topic"},
    )

    messagesConsumed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mq_messages_consumed_total",
            Help: "Total messages consumed",
        },
        []string{"topic", "consumer_group"},
    )

    messageLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "mq_message_latency_seconds",
            Help:    "Message processing latency",
            Buckets: prometheus.DefBuckets,
        },
        []string{"topic"},
    )
)
```

### 15.3 Health Checks

```go
// Liveness - is the service running?
func (h *Handler) LivenessCheck(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

// Readiness - is the service ready to accept traffic?
func (h *Handler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
    // Check database connection
    if err := h.db.Ping(r.Context()); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    
    // Check MQ connection
    if !h.mqClient.IsConnected() {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}
```

---

## 16. Security Considerations

### 16.1 Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: api-gateway-policy
spec:
  podSelector:
    matchLabels:
      app: api-gateway
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from: []  # Allow external traffic
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: postgresql
    ports:
    - protocol: TCP
      port: 5432
```

### 16.2 Secrets Management

- Database credentials stored in Kubernetes Secrets
- Secrets mounted as environment variables
- Consider using external secret management (Vault) for production

---

## 17. Technology Decisions

### 17.1 Why These Choices?

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Required by spec, great for systems programming |
| HTTP Framework | chi | Lightweight, idiomatic, good middleware support |
| Database | PostgreSQL | Robust, good for time-series, JSON support |
| ORM | sqlx | Not an ORM, but great SQL builder with struct scanning |
| OpenAPI | swaggo/swag | Auto-generates from Go annotations |
| Logging | zerolog | Fast, structured, zero-allocation |
| Config | Viper | Standard Go config library |
| Testing | testify | Assertions and mocking |

### 17.2 Database Choice: PostgreSQL vs Alternatives

#### Why PostgreSQL Was Chosen

| Criteria | PostgreSQL | MongoDB |
|----------|------------|---------|
| **Schema** | ✅ Fixed schema - telemetry data has predictable structure | ❌ Schema-less adds overhead for validation |
| **Time-Range Queries** | ✅ Excellent with B-tree indexes | ⚠️ Works but less optimized |
| **SQL Aggregations** | ✅ Mature, powerful (AVG, GROUP BY, date_trunc) | ⚠️ Aggregation pipeline more complex |
| **K8s Deployment** | ✅ Single StatefulSet | ⚠️ Needs replica set (3+ nodes minimum) |
| **Memory Footprint** | ✅ Lower | ⚠️ Higher memory requirements |
| **Time-Series Extensions** | ✅ TimescaleDB available | ⚠️ Time-series collections (newer feature) |

#### Query Pattern Analysis

The primary API query:
```
GET /api/v1/gpus/{id}/telemetry?start_time=X&end_time=Y
```

**PostgreSQL** (optimal for this pattern):
```sql
SELECT * FROM gpu_telemetry 
WHERE gpu_uuid = $1 
  AND timestamp BETWEEN $2 AND $3
ORDER BY timestamp;
-- Uses composite B-tree index efficiently
```

**MongoDB** (works but less optimal):
```javascript
db.telemetry.find({
  gpu_uuid: "...",
  timestamp: { $gte: start, $lte: end }
}).sort({ timestamp: 1 })
```

#### When MongoDB Would Be Better

MongoDB would be preferred if:
- Telemetry schema varied per GPU vendor (heterogeneous data)
- Complex nested objects needed to be stored
- Horizontal sharding across many nodes was required
- Document-oriented queries were primary use case

#### Decision Summary

For fixed-schema, time-series GPU telemetry with range queries and a limit of 10 streamer/collector instances, **PostgreSQL is the optimal choice**.

### 17.3 Other Alternative Considerations

| Component | Chosen | Alternative | Why Not |
|-----------|--------|-------------|---------|
| Database | PostgreSQL | SQLite | Need proper concurrent write access |
| Database | PostgreSQL | MongoDB | Fixed schema, time-range queries favor SQL |
| MQ Protocol | gRPC (8081) | HTTP only | gRPC provides efficient binary protocol, type-safety via protobuf |
| MQ Admin API | HTTP (8082) | gRPC only | HTTP easier for debugging, Swagger UI, curl testing |
| Message Format | Protobuf (gRPC) + JSON (Admin) | JSON only | Protobuf for efficiency, JSON for admin readability |
| HTTP Framework | chi | gin | chi is more lightweight and idiomatic |

---

## Appendix A: Project Structure

```
gpu-telemetry-pipeline/
├── cmd/
│   ├── streamer/
│   │   └── main.go
│   ├── collector/
│   │   └── main.go
│   ├── api-gateway/
│   │   └── main.go
│   └── mq-broker/
│       └── main.go
├── internal/
│   ├── streamer/
│   │   ├── streamer.go
│   │   ├── csv_reader.go
│   │   └── streamer_test.go
│   ├── collector/
│   │   ├── collector.go
│   │   ├── processor.go
│   │   └── collector_test.go
│   ├── api/
│   │   ├── handler.go
│   │   ├── routes.go
│   │   └── handler_test.go
│   ├── mq/
│   │   ├── broker/
│   │   │   ├── broker.go
│   │   │   ├── topic.go
│   │   │   ├── consumer_group.go
│   │   │   └── broker_test.go
│   │   └── client/
│   │       ├── client.go
│   │       ├── publisher.go
│   │       ├── subscriber.go
│   │       └── client_test.go
│   ├── storage/
│   │   ├── postgres/
│   │   │   ├── repository.go
│   │   │   └── repository_test.go
│   │   └── migrations/
│   │       └── 001_initial.sql
│   └── models/
│       ├── telemetry.go
│       └── gpu.go
├── pkg/
│   └── config/
│       └── config.go
├── api/
│   └── openapi/
│       └── swagger.yaml (auto-generated)
├── deployments/
│   ├── docker/
│   │   ├── Dockerfile.streamer
│   │   ├── Dockerfile.collector
│   │   ├── Dockerfile.api-gateway
│   │   └── Dockerfile.mq-broker
│   └── helm/
│       └── gpu-telemetry/
│           ├── Chart.yaml
│           ├── values.yaml
│           └── templates/
├── data/
│   └── dcgm_metrics.csv
├── scripts/
│   ├── setup-kind.sh
│   └── generate-swagger.sh
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## Appendix B: Makefile Targets

```makefile
.PHONY: all build test docker helm deploy

# Build all binaries
build:
	go build -o bin/streamer ./cmd/streamer
	go build -o bin/collector ./cmd/collector
	go build -o bin/api-gateway ./cmd/api-gateway
	go build -o bin/mq-broker ./cmd/mq-broker

# Run all tests with coverage
test:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Generate OpenAPI spec
swagger:
	swag init -g cmd/api-gateway/main.go -o api/openapi

# Build Docker images
docker:
	docker build -f deployments/docker/Dockerfile.streamer -t streamer:latest .
	docker build -f deployments/docker/Dockerfile.collector -t collector:latest .
	docker build -f deployments/docker/Dockerfile.api-gateway -t api-gateway:latest .
	docker build -f deployments/docker/Dockerfile.mq-broker -t mq-broker:latest .

# Create Kind cluster
kind-create:
	kind create cluster --name gpu-telemetry --config=deployments/kind-config.yaml

# Load images to Kind
kind-load: docker
	kind load docker-image streamer:latest --name gpu-telemetry
	kind load docker-image collector:latest --name gpu-telemetry
	kind load docker-image api-gateway:latest --name gpu-telemetry
	kind load docker-image mq-broker:latest --name gpu-telemetry

# Deploy to Kubernetes
deploy:
	helm upgrade --install gpu-telemetry ./deployments/helm/gpu-telemetry

# Full workflow
all: build test docker kind-load deploy
```

---

*Document Version: 2.0*  
*Last Updated: January 2026*

---

## Changelog

### Version 2.0 (January 2026)
- Added **Dead Letter Queue (DLQ)** design with auto-replay mechanism
- Added **Deployment Order & Dependencies** section with init containers
- Added **Admission Controller (Kyverno)** for enforcing scaling limits
- Updated **Scaling Strategy** with detailed constraints for MQ and PostgreSQL
- Updated MQ to show **dual protocol** (gRPC + HTTP)
- Added PostgreSQL concurrency explanation with multiple Collectors/Gateways

### Version 1.0 (January 2025)
- Initial design document
