package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gpu-telemetry-pipeline/collector/internal/repository"
	"github.com/gpu-telemetry-pipeline/collector/pkg/pb"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.NumWorkers != 3 {
		t.Errorf("Expected 3 workers, got %d", cfg.NumWorkers)
	}

	if cfg.BufferSize != 10 {
		t.Errorf("Expected buffer size 10, got %d", cfg.BufferSize)
	}
}

func TestNewPool(t *testing.T) {
	logger := zerolog.Nop()
	repo := repository.NewNoOpRepository(logger)

	cfg := Config{NumWorkers: 5, BufferSize: 20}
	pool := NewPool(cfg, repo, logger)

	if pool == nil {
		t.Fatal("Expected pool to be created")
	}
}

func TestNewPool_DefaultsOnInvalidConfig(t *testing.T) {
	logger := zerolog.Nop()
	repo := repository.NewNoOpRepository(logger)

	cfg := Config{NumWorkers: 0, BufferSize: 0}
	pool := NewPool(cfg, repo, logger)

	if pool == nil {
		t.Fatal("Expected pool to be created")
	}
}

func TestPool_StartStop(t *testing.T) {
	logger := zerolog.Nop()
	repo := repository.NewNoOpRepository(logger)

	cfg := Config{NumWorkers: 2, BufferSize: 5}
	pool := NewPool(cfg, repo, logger)

	ctx := context.Background()
	pool.Start(ctx)

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	pool.Stop()
}

func createTestMessage(id string, payload string) *pb.Message {
	return &pb.Message{
		Id:        id,
		Topic:     "test-topic",
		Payload:   []byte(payload),
		Timestamp: timestamppb.Now(),
	}
}

func TestPool_Submit(t *testing.T) {
	logger := zerolog.Nop()
	repo := repository.NewNoOpRepository(logger)

	cfg := Config{NumWorkers: 2, BufferSize: 5}
	pool := NewPool(cfg, repo, logger)

	ctx := context.Background()
	pool.Start(ctx)
	defer pool.Stop()

	payload := `{"timestamp":"2026-01-28T15:30:00Z","metric_name":"GPU_UTIL","gpu_id":"0","uuid":"GPU-123","Hostname":"host1","value":"50"}`

	workItem := WorkItem{
		Messages: []*pb.Message{
			createTestMessage("msg-1", payload),
			createTestMessage("msg-2", payload),
		},
	}

	submitted := pool.Submit(ctx, workItem)
	if !submitted {
		t.Error("Expected work item to be submitted")
	}

	// Wait for result
	select {
	case result := <-pool.Results():
		if result.Error != nil {
			t.Errorf("Unexpected error: %v", result.Error)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for result")
	}
}

func TestPool_Stats(t *testing.T) {
	logger := zerolog.Nop()
	repo := repository.NewNoOpRepository(logger)

	cfg := Config{NumWorkers: 2, BufferSize: 5}
	pool := NewPool(cfg, repo, logger)

	ctx := context.Background()
	pool.Start(ctx)

	payload := `{"timestamp":"2026-01-28T15:30:00Z","metric_name":"GPU_UTIL","gpu_id":"0","uuid":"GPU-123","Hostname":"host1","value":"50"}`
	pool.Submit(ctx, WorkItem{Messages: []*pb.Message{createTestMessage("msg-1", payload)}})

	// Wait for processing
	<-pool.Results()

	// Give a moment for stats to update
	time.Sleep(10 * time.Millisecond)

	stats := pool.GetStats()
	// Verify basic stats are available
	if stats.NumWorkers != 2 {
		t.Errorf("Expected 2 workers, got %d", stats.NumWorkers)
	}

	pool.Stop()
}

func TestPool_MultipleWorkItems(t *testing.T) {
	logger := zerolog.Nop()
	repo := repository.NewNoOpRepository(logger)

	cfg := Config{NumWorkers: 3, BufferSize: 20}
	pool := NewPool(cfg, repo, logger)

	ctx := context.Background()
	pool.Start(ctx)
	defer pool.Stop()

	var completed atomic.Int32
	payload := `{"timestamp":"2026-01-28T15:30:00Z","metric_name":"GPU_UTIL","gpu_id":"0","uuid":"GPU-123","Hostname":"host1","value":"50"}`

	// Submit multiple work items
	numItems := 5
	for i := 0; i < numItems; i++ {
		pool.Submit(ctx, WorkItem{
			Messages: []*pb.Message{
				createTestMessage("msg-"+string(rune('a'+i)), payload),
			},
		})
	}

	// Collect results with timeout
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := 0; i < numItems; i++ {
		select {
		case <-pool.Results():
			completed.Add(1)
		case <-timeoutCtx.Done():
			t.Fatalf("Timeout: only completed %d items", completed.Load())
		}
	}

	if completed.Load() != int32(numItems) {
		t.Errorf("Expected %d completed, got %d", numItems, completed.Load())
	}
}

func TestPool_InvalidPayload(t *testing.T) {
	logger := zerolog.Nop()
	repo := repository.NewNoOpRepository(logger)

	cfg := Config{NumWorkers: 1, BufferSize: 5}
	pool := NewPool(cfg, repo, logger)

	ctx := context.Background()
	pool.Start(ctx)
	defer pool.Stop()

	// Submit invalid JSON payload
	pool.Submit(ctx, WorkItem{
		Messages: []*pb.Message{
			createTestMessage("msg-invalid", `{invalid json}`),
		},
	})

	// Should still get a result (possibly with error or empty records)
	select {
	case result := <-pool.Results():
		// The pool should handle invalid payloads gracefully
		_ = result
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for result")
	}
}

func TestPool_EmptyMessages(t *testing.T) {
	logger := zerolog.Nop()
	repo := repository.NewNoOpRepository(logger)

	cfg := Config{NumWorkers: 1, BufferSize: 5}
	pool := NewPool(cfg, repo, logger)

	ctx := context.Background()
	pool.Start(ctx)
	defer pool.Stop()

	// Submit empty work item
	pool.Submit(ctx, WorkItem{Messages: []*pb.Message{}})

	select {
	case result := <-pool.Results():
		if result.Count != 0 {
			t.Errorf("Expected 0 count for empty messages, got %d", result.Count)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for result")
	}
}

func TestPool_Results(t *testing.T) {
	logger := zerolog.Nop()
	repo := repository.NewNoOpRepository(logger)

	cfg := Config{NumWorkers: 1, BufferSize: 5}
	pool := NewPool(cfg, repo, logger)

	results := pool.Results()
	if results == nil {
		t.Fatal("Expected Results channel to be created")
	}
}
