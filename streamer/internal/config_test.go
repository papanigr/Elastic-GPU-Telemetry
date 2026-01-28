package internal

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear env vars
	os.Unsetenv("STREAMER_CSV_FILE_PATH")
	os.Unsetenv("STREAMER_MQ_ENABLED")
	os.Unsetenv("STREAMER_STREAM_INTERVAL")
	os.Unsetenv("STREAMER_BATCH_SIZE")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.CSVFilePath != "/data/dcgm_metrics.csv" {
		t.Errorf("Expected default CSVFilePath '/data/dcgm_metrics.csv', got %s", cfg.CSVFilePath)
	}

	if cfg.MQEnabled != true {
		t.Errorf("Expected default MQEnabled true, got %v", cfg.MQEnabled)
	}

	if cfg.MQBrokerAddr != "mq-broker:8081" {
		t.Errorf("Expected default MQBrokerAddr 'mq-broker:8081', got %s", cfg.MQBrokerAddr)
	}

	if cfg.Topic != "gpu-telemetry" {
		t.Errorf("Expected default Topic 'gpu-telemetry', got %s", cfg.Topic)
	}

	if cfg.StreamInterval != 5*time.Second {
		t.Errorf("Expected default StreamInterval 5s, got %v", cfg.StreamInterval)
	}

	if cfg.BatchSize != 10 {
		t.Errorf("Expected default BatchSize 10, got %d", cfg.BatchSize)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default LogLevel 'info', got %s", cfg.LogLevel)
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	os.Setenv("STREAMER_CSV_FILE_PATH", "/custom/path.csv")
	os.Setenv("STREAMER_MQ_ENABLED", "false")
	os.Setenv("STREAMER_MQ_BROKER_ADDR", "localhost:9999")
	os.Setenv("STREAMER_TOPIC", "custom-topic")
	os.Setenv("STREAMER_STREAM_INTERVAL", "10s")
	os.Setenv("STREAMER_BATCH_SIZE", "50")
	os.Setenv("STREAMER_FULL_FILE_BATCH", "true")
	os.Setenv("STREAMER_LOG_LEVEL", "debug")

	defer func() {
		os.Unsetenv("STREAMER_CSV_FILE_PATH")
		os.Unsetenv("STREAMER_MQ_ENABLED")
		os.Unsetenv("STREAMER_MQ_BROKER_ADDR")
		os.Unsetenv("STREAMER_TOPIC")
		os.Unsetenv("STREAMER_STREAM_INTERVAL")
		os.Unsetenv("STREAMER_BATCH_SIZE")
		os.Unsetenv("STREAMER_FULL_FILE_BATCH")
		os.Unsetenv("STREAMER_LOG_LEVEL")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.CSVFilePath != "/custom/path.csv" {
		t.Errorf("Expected CSVFilePath '/custom/path.csv', got %s", cfg.CSVFilePath)
	}

	if cfg.MQEnabled {
		t.Error("Expected MQEnabled to be false")
	}

	if cfg.MQBrokerAddr != "localhost:9999" {
		t.Errorf("Expected MQBrokerAddr 'localhost:9999', got %s", cfg.MQBrokerAddr)
	}

	if cfg.Topic != "custom-topic" {
		t.Errorf("Expected Topic 'custom-topic', got %s", cfg.Topic)
	}

	if cfg.StreamInterval != 10*time.Second {
		t.Errorf("Expected StreamInterval 10s, got %v", cfg.StreamInterval)
	}

	if cfg.BatchSize != 50 {
		t.Errorf("Expected BatchSize 50, got %d", cfg.BatchSize)
	}

	if !cfg.FullFileBatch {
		t.Error("Expected FullFileBatch to be true")
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug', got %s", cfg.LogLevel)
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.CSVFilePath != "/data/dcgm_metrics.csv" {
		t.Errorf("Expected CSVFilePath '/data/dcgm_metrics.csv', got %s", cfg.CSVFilePath)
	}

	if cfg.BatchSize != 10 {
		t.Errorf("Expected BatchSize 10, got %d", cfg.BatchSize)
	}

	if cfg.StreamInterval != 5*time.Second {
		t.Errorf("Expected StreamInterval 5s, got %v", cfg.StreamInterval)
	}

	if cfg.MQEnabled != true {
		t.Error("Expected MQEnabled to be true by default")
	}
}
