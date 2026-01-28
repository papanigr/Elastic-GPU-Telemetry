package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.GRPCPort != 8081 {
		t.Errorf("Expected GRPCPort 8081, got %d", cfg.GRPCPort)
	}

	if cfg.HTTPPort != 8082 {
		t.Errorf("Expected HTTPPort 8082, got %d", cfg.HTTPPort)
	}

	if cfg.MaxQueueSize != 10000 {
		t.Errorf("Expected MaxQueueSize 10000, got %d", cfg.MaxQueueSize)
	}

	if cfg.MaxMessageAge != 5*time.Minute {
		t.Errorf("Expected MaxMessageAge 5m, got %v", cfg.MaxMessageAge)
	}

	if cfg.AckTimeout != 30*time.Second {
		t.Errorf("Expected AckTimeout 30s, got %v", cfg.AckTimeout)
	}

	if cfg.CleanupInterval != 10*time.Second {
		t.Errorf("Expected CleanupInterval 10s, got %v", cfg.CleanupInterval)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected LogLevel 'info', got %s", cfg.LogLevel)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear env vars
	os.Unsetenv("MQ_GRPC_PORT")
	os.Unsetenv("MQ_HTTP_PORT")
	os.Unsetenv("MQ_MAX_QUEUE_SIZE")
	os.Unsetenv("MQ_LOG_LEVEL")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.GRPCPort != 8081 {
		t.Errorf("Expected GRPCPort 8081, got %d", cfg.GRPCPort)
	}

	if cfg.HTTPPort != 8082 {
		t.Errorf("Expected HTTPPort 8082, got %d", cfg.HTTPPort)
	}
}

func TestLoadConfig_FromEnv(t *testing.T) {
	os.Setenv("MQ_GRPC_PORT", "9091")
	os.Setenv("MQ_HTTP_PORT", "9092")
	os.Setenv("MQ_MAX_QUEUE_SIZE", "50000")
	os.Setenv("MQ_MAX_MESSAGE_AGE", "10m")
	os.Setenv("MQ_ACK_TIMEOUT", "60s")
	os.Setenv("MQ_CLEANUP_INTERVAL", "30s")
	os.Setenv("MQ_LOG_LEVEL", "debug")

	defer func() {
		os.Unsetenv("MQ_GRPC_PORT")
		os.Unsetenv("MQ_HTTP_PORT")
		os.Unsetenv("MQ_MAX_QUEUE_SIZE")
		os.Unsetenv("MQ_MAX_MESSAGE_AGE")
		os.Unsetenv("MQ_ACK_TIMEOUT")
		os.Unsetenv("MQ_CLEANUP_INTERVAL")
		os.Unsetenv("MQ_LOG_LEVEL")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.GRPCPort != 9091 {
		t.Errorf("Expected GRPCPort 9091, got %d", cfg.GRPCPort)
	}

	if cfg.HTTPPort != 9092 {
		t.Errorf("Expected HTTPPort 9092, got %d", cfg.HTTPPort)
	}

	if cfg.MaxQueueSize != 50000 {
		t.Errorf("Expected MaxQueueSize 50000, got %d", cfg.MaxQueueSize)
	}

	if cfg.MaxMessageAge != 10*time.Minute {
		t.Errorf("Expected MaxMessageAge 10m, got %v", cfg.MaxMessageAge)
	}

	if cfg.AckTimeout != 60*time.Second {
		t.Errorf("Expected AckTimeout 60s, got %v", cfg.AckTimeout)
	}

	if cfg.CleanupInterval != 30*time.Second {
		t.Errorf("Expected CleanupInterval 30s, got %v", cfg.CleanupInterval)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected LogLevel 'debug', got %s", cfg.LogLevel)
	}
}
