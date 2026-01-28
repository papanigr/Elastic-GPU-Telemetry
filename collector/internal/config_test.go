package internal

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear env vars
	os.Unsetenv("COLLECTOR_MQ_BROKER_ADDR")
	os.Unsetenv("COLLECTOR_TOPIC")
	os.Unsetenv("COLLECTOR_CONSUMER_GROUP")
	os.Unsetenv("COLLECTOR_BATCH_SIZE")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.MQBrokerAddr != "mq-broker:8081" {
		t.Errorf("Expected default MQBrokerAddr 'mq-broker:8081', got %s", cfg.MQBrokerAddr)
	}

	if cfg.Topic != "gpu-telemetry" {
		t.Errorf("Expected default topic 'gpu-telemetry', got %s", cfg.Topic)
	}

	if cfg.ConsumerGroup != "telemetry-collectors" {
		t.Errorf("Expected default consumer group 'telemetry-collectors', got %s", cfg.ConsumerGroup)
	}

	if cfg.BatchSize != 10 {
		t.Errorf("Expected default batch size 10, got %d", cfg.BatchSize)
	}

	if cfg.PollInterval != 1*time.Second {
		t.Errorf("Expected default poll interval 1s, got %v", cfg.PollInterval)
	}

	if cfg.NumWorkers != 3 {
		t.Errorf("Expected default num workers 3, got %d", cfg.NumWorkers)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got %s", cfg.LogLevel)
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("COLLECTOR_MQ_BROKER_ADDR", "localhost:9999")
	os.Setenv("COLLECTOR_TOPIC", "custom-topic")
	os.Setenv("COLLECTOR_CONSUMER_GROUP", "custom-group")
	os.Setenv("COLLECTOR_BATCH_SIZE", "50")
	os.Setenv("COLLECTOR_POLL_INTERVAL", "2s")
	os.Setenv("COLLECTOR_NUM_WORKERS", "10")
	os.Setenv("COLLECTOR_DB_ENABLED", "true")
	os.Setenv("COLLECTOR_DATABASE_URL", "postgres://user:pass@localhost/db")
	os.Setenv("COLLECTOR_LOG_LEVEL", "debug")

	defer func() {
		os.Unsetenv("COLLECTOR_MQ_BROKER_ADDR")
		os.Unsetenv("COLLECTOR_TOPIC")
		os.Unsetenv("COLLECTOR_CONSUMER_GROUP")
		os.Unsetenv("COLLECTOR_BATCH_SIZE")
		os.Unsetenv("COLLECTOR_POLL_INTERVAL")
		os.Unsetenv("COLLECTOR_NUM_WORKERS")
		os.Unsetenv("COLLECTOR_DB_ENABLED")
		os.Unsetenv("COLLECTOR_DATABASE_URL")
		os.Unsetenv("COLLECTOR_LOG_LEVEL")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.MQBrokerAddr != "localhost:9999" {
		t.Errorf("Expected MQBrokerAddr 'localhost:9999', got %s", cfg.MQBrokerAddr)
	}

	if cfg.Topic != "custom-topic" {
		t.Errorf("Expected topic 'custom-topic', got %s", cfg.Topic)
	}

	if cfg.ConsumerGroup != "custom-group" {
		t.Errorf("Expected consumer group 'custom-group', got %s", cfg.ConsumerGroup)
	}

	if cfg.BatchSize != 50 {
		t.Errorf("Expected batch size 50, got %d", cfg.BatchSize)
	}

	if cfg.PollInterval != 2*time.Second {
		t.Errorf("Expected poll interval 2s, got %v", cfg.PollInterval)
	}

	if cfg.NumWorkers != 10 {
		t.Errorf("Expected num workers 10, got %d", cfg.NumWorkers)
	}

	if !cfg.DBEnabled {
		t.Error("Expected DBEnabled to be true")
	}

	if cfg.DatabaseURL != "postgres://user:pass@localhost/db" {
		t.Errorf("Expected DatabaseURL 'postgres://user:pass@localhost/db', got %s", cfg.DatabaseURL)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got %s", cfg.LogLevel)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Verify basic defaults
	if cfg.MQBrokerAddr != "mq-broker:8081" {
		t.Errorf("Expected MQBrokerAddr 'mq-broker:8081', got %s", cfg.MQBrokerAddr)
	}

	if cfg.Topic != "gpu-telemetry" {
		t.Errorf("Expected Topic 'gpu-telemetry', got %s", cfg.Topic)
	}

	if cfg.BatchSize != 10 {
		t.Errorf("Expected BatchSize 10, got %d", cfg.BatchSize)
	}

	if cfg.NumWorkers != 3 {
		t.Errorf("Expected NumWorkers 3, got %d", cfg.NumWorkers)
	}
}
