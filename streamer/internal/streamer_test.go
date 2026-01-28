package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gpu-telemetry-pipeline/streamer/pkg/models"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPublisher is a mock implementation of the Publisher interface for testing.
type mockPublisher struct {
	publishedRecords []models.TelemetryRecord
	publishError     error
	closed           bool
}

func (m *mockPublisher) Publish(ctx context.Context, topic string, record models.TelemetryRecord) error {
	if m.publishError != nil {
		return m.publishError
	}
	m.publishedRecords = append(m.publishedRecords, record)
	return nil
}

func (m *mockPublisher) Close() error {
	m.closed = true
	return nil
}

func TestNew(t *testing.T) {
	// Create a temporary CSV file
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test_metrics.csv")

	csvContent := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_UTIL","0","nvidia0","GPU-5fd4f087","NVIDIA H100","host1","","","","42.5",""
`
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	logger := zerolog.Nop()

	t.Run("creates streamer with MQ disabled", func(t *testing.T) {
		config := Config{
			CSVFilePath:    csvPath,
			MQEnabled:      false,
			Topic:          "test-topic",
			StreamInterval: 100 * time.Millisecond,
			BatchSize:      1,
		}

		s, err := New(config, logger)
		require.NoError(t, err)
		require.NotNil(t, s)
		defer s.Close()
	})

	t.Run("fails with invalid CSV path", func(t *testing.T) {
		config := Config{
			CSVFilePath:    "/non/existent/file.csv",
			MQEnabled:      false,
			Topic:          "test-topic",
			StreamInterval: 100 * time.Millisecond,
			BatchSize:      1,
		}

		s, err := New(config, logger)
		assert.Error(t, err)
		assert.Nil(t, s)
	})
}

func TestStreamer_StreamBatch(t *testing.T) {
	// Create a temporary CSV file
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test_metrics.csv")

	csvContent := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_UTIL","0","nvidia0","GPU-5fd4f087","NVIDIA H100","host1","","","","42.5",""
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_TEMP","1","nvidia1","GPU-bc7a12ab","NVIDIA H100","host1","","","","75.0",""
"2025-07-18T20:42:34Z","DCGM_FI_DEV_POWER_USAGE","2","nvidia2","GPU-6c8915d7","NVIDIA H100","host1","","","","300.0",""
`
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	logger := zerolog.Nop()

	t.Run("streams single record batch", func(t *testing.T) {
		config := Config{
			CSVFilePath:    csvPath,
			MQEnabled:      false,
			Topic:          "test-topic",
			StreamInterval: 100 * time.Millisecond,
			BatchSize:      1,
		}

		s, err := New(config, logger)
		require.NoError(t, err)
		defer s.Close()

		// Replace publisher with mock
		mock := &mockPublisher{}
		s.SetPublisher(mock)

		ctx := context.Background()
		err = s.streamBatch(ctx)
		require.NoError(t, err)

		assert.Len(t, mock.publishedRecords, 1)
		assert.Equal(t, "DCGM_FI_DEV_GPU_UTIL", mock.publishedRecords[0].MetricName)
	})

	t.Run("streams multiple record batch", func(t *testing.T) {
		config := Config{
			CSVFilePath:    csvPath,
			MQEnabled:      false,
			Topic:          "test-topic",
			StreamInterval: 100 * time.Millisecond,
			BatchSize:      3,
		}

		s, err := New(config, logger)
		require.NoError(t, err)
		defer s.Close()

		// Replace publisher with mock
		mock := &mockPublisher{}
		s.SetPublisher(mock)

		ctx := context.Background()
		err = s.streamBatch(ctx)
		require.NoError(t, err)

		assert.Len(t, mock.publishedRecords, 3)
		assert.Equal(t, "DCGM_FI_DEV_GPU_UTIL", mock.publishedRecords[0].MetricName)
		assert.Equal(t, "DCGM_FI_DEV_GPU_TEMP", mock.publishedRecords[1].MetricName)
		assert.Equal(t, "DCGM_FI_DEV_POWER_USAGE", mock.publishedRecords[2].MetricName)
	})
}

func TestStreamer_Run(t *testing.T) {
	// Create a temporary CSV file
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test_metrics.csv")

	csvContent := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_UTIL","0","nvidia0","GPU-5fd4f087","NVIDIA H100","host1","","","","42.5",""
`
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	logger := zerolog.Nop()

	t.Run("runs and stops on context cancellation", func(t *testing.T) {
		config := Config{
			CSVFilePath:    csvPath,
			MQEnabled:      false,
			Topic:          "test-topic",
			StreamInterval: 10 * time.Millisecond,
			BatchSize:      1,
		}

		s, err := New(config, logger)
		require.NoError(t, err)
		defer s.Close()

		// Replace publisher with mock
		mock := &mockPublisher{}
		s.SetPublisher(mock)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err = s.Run(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)

		// Should have streamed some records
		assert.Greater(t, len(mock.publishedRecords), 0)
	})
}

func TestStreamer_GetStats(t *testing.T) {
	// Create a temporary CSV file
	tmpDir := t.TempDir()
	csvPath := filepath.Join(tmpDir, "test_metrics.csv")

	csvContent := `timestamp,metric_name,gpu_id,device,uuid,modelName,Hostname,container,pod,namespace,value,labels_raw
"2025-07-18T20:42:34Z","DCGM_FI_DEV_GPU_UTIL","0","nvidia0","GPU-5fd4f087","NVIDIA H100","host1","","","","42.5",""
`
	err := os.WriteFile(csvPath, []byte(csvContent), 0644)
	require.NoError(t, err)

	logger := zerolog.Nop()

	config := Config{
		CSVFilePath:    csvPath,
		MQEnabled:      false,
		Topic:          "test-topic",
		StreamInterval: 10 * time.Millisecond,
		BatchSize:      1,
	}

	s, err := New(config, logger)
	require.NoError(t, err)
	defer s.Close()

	// Replace publisher with mock
	mock := &mockPublisher{}
	s.SetPublisher(mock)

	// Run for a short time
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_ = s.Run(ctx)

	stats := s.GetStats()
	assert.Greater(t, stats.RecordsStreamed, int64(0))
	assert.Equal(t, int64(0), stats.ErrorsCount)
	assert.Greater(t, stats.Uptime, time.Duration(0))
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "/data/dcgm_metrics.csv", config.CSVFilePath)
	assert.Equal(t, "mq-broker:8081", config.MQBrokerAddr)
	assert.Equal(t, "gpu-telemetry", config.Topic)
	assert.Equal(t, 5*time.Second, config.StreamInterval)
	assert.Equal(t, 10, config.BatchSize)
	assert.True(t, config.MQEnabled)
	assert.Equal(t, "info", config.LogLevel)
}
